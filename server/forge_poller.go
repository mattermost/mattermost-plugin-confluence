// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

const forgePollerJobKey = "forge_drain_poller"

const (
	forgePollInterval = 30 * time.Second
	forgePollTimeout  = 20 * time.Second
	forgeDrainBatch   = 100
)

// ForgePoller drains events from the Confluence Forge bridge. In a clustered
// deployment, cluster.Schedule guarantees only one node runs the poll loop at
// any time via a distributed mutex.
type ForgePoller struct {
	plugin *Plugin
	client *http.Client

	mu  sync.Mutex
	job *cluster.Job
}

func NewForgePoller(p *Plugin) *ForgePoller {
	return &ForgePoller{
		plugin: p,
		client: &http.Client{
			Timeout: forgePollTimeout,
			// Refuse redirects so the HMAC-signed body stays on the validated host.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (fp *ForgePoller) Start() {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if fp.job != nil {
		return
	}
	job, err := cluster.Schedule(
		fp.plugin.API,
		forgePollerJobKey,
		cluster.MakeWaitForInterval(forgePollInterval),
		func() {
			if err := fp.drainOnce(context.Background()); err != nil {
				fp.plugin.client.Log.Debug("forge drain failed", "error", err.Error())
			}
		},
	)
	if err != nil {
		fp.plugin.client.Log.Error("failed to schedule forge drain poller", "error", err.Error())
		return
	}
	fp.job = job
}

func (fp *ForgePoller) Stop() {
	fp.mu.Lock()
	job := fp.job
	fp.job = nil
	fp.mu.Unlock()
	if job == nil {
		return
	}
	if err := job.Close(); err != nil {
		fp.plugin.client.Log.Warn("failed to stop forge drain poller", "error", err.Error())
	}
}

type forgeDrainRequest struct {
	Limit int      `json:"limit"`
	Ack   []string `json:"ack,omitempty"`
}

type forgeDrainEvent struct {
	Key   string        `json:"key"`
	Value forgeEnvelope `json:"value"`
}

type forgeEnvelope struct {
	Event      json.RawMessage `json:"event"`
	Context    json.RawMessage `json:"context"`
	EnqueuedAt int64           `json:"enqueuedAt"`
}

type forgeDrainResponse struct {
	Events     []forgeDrainEvent `json:"events"`
	NextCursor *string           `json:"nextCursor"`
}

func (fp *ForgePoller) drainOnce(ctx context.Context) error {
	cfg := config.GetConfig()
	if cfg.ForgeDrainURL == "" || cfg.ForgeSharedSecret == "" {
		return nil // not configured, nothing to do
	}

	body, err := json.Marshal(forgeDrainRequest{Limit: forgeDrainBatch})
	if err != nil {
		return errors.Wrap(err, "marshal drain request")
	}

	resp, err := fp.post(ctx, cfg.ForgeDrainURL, cfg.ForgeSharedSecret, body)
	if err != nil {
		return err
	}

	if len(resp.Events) == 0 {
		return nil
	}

	ackKeys := make([]string, 0, len(resp.Events))
	for _, evt := range resp.Events {
		if err := fp.dispatch(evt); err != nil {
			fp.plugin.client.Log.Warn("forge drain: dropping event", "key", evt.Key, "error", err.Error())
		}
		ackKeys = append(ackKeys, evt.Key)
	}

	ackBody, err := json.Marshal(forgeDrainRequest{Limit: 0, Ack: ackKeys})
	if err != nil {
		return errors.Wrap(err, "marshal ack request")
	}
	if _, err := fp.post(ctx, cfg.ForgeDrainURL, cfg.ForgeSharedSecret, ackBody); err != nil {
		return errors.Wrap(err, "ack drained events")
	}
	return nil
}

func (fp *ForgePoller) post(ctx context.Context, url, secret string, body []byte) (*forgeDrainResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "build drain request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MM-Signature", signForgeBody(secret, body))

	resp, err := fp.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "drain request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("drain returned %d: %s", resp.StatusCode, string(raw))
	}

	var out forgeDrainResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, errors.Wrap(err, "decode drain response")
	}
	return &out, nil
}

func (fp *ForgePoller) dispatch(evt forgeDrainEvent) error {
	forgeEvent, err := serializer.ForgeEventFromJSON(bytes.NewReader(evt.Value.Event))
	if err != nil {
		return errors.Wrap(err, "deserialize forge event")
	}

	internal, ok := forgeToInternalEvent[forgeEvent.EventType]
	if !ok {
		return fmt.Errorf("unmapped forge event type %q", forgeEvent.EventType)
	}

	forgeEvent.BaseURL = config.GetConfig().GetConfluenceBaseURL()

	go service.SendConfluenceNotifications(forgeEvent, internal)
	go fp.dispatchMentionDMs(forgeEvent, internal)
	return nil
}

func (fp *ForgePoller) dispatchMentionDMs(evt *serializer.ForgeEvent, internalEvent string) {
	if evt == nil || evt.Content == nil || !mentionEligibleEvent(internalEvent) {
		return
	}

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	client, err := fp.plugin.cloudClientForActor(instanceURL, evt.Context.CloudID, evt.ActorAccountID())
	if err != nil {
		fp.plugin.client.Log.Debug("mention DM: no cloud client", "error", err.Error())
		return
	}

	var (
		kind       service.ContentKind
		accountIDs []string
	)
	if evt.Content.Type == "comment" {
		kind = service.ContentKindComment
		accountIDs, err = client.MentionAccountIDsInComment(evt.Content.ID.String(), evt.CommentLocation())
	} else {
		kind = service.ContentKindPage
		accountIDs, err = client.MentionAccountIDsInPage(evt.Content.ID.String())
	}
	if err != nil {
		if errors.Is(err, ErrCloudInsufficientScope) {
			fp.notifyActorToReconnect(instanceURL, evt.ActorAccountID())
		}
		fp.plugin.client.Log.Debug("mention DM: failed to fetch mentions", "error", err.Error())
		return
	}
	if len(accountIDs) == 0 {
		return
	}

	pageTitle, pageURL := evt.MentionPageContext()
	service.SendMentionDMs(service.MentionDispatchParams{
		InstanceID:     instanceURL,
		AccountIDs:     accountIDs,
		Kind:           kind,
		PageTitle:      pageTitle,
		PageURL:        pageURL,
		ActorAccountID: evt.ActorAccountID(),
	})
}

func (fp *ForgePoller) notifyActorToReconnect(instanceURL, actorAccountID string) {
	if actorAccountID == "" {
		return
	}
	mmUserIDPtr, err := store.GetMattermostUserIDFromConfluenceID(instanceURL, actorAccountID)
	if err != nil || mmUserIDPtr == nil || *mmUserIDPtr == "" {
		return
	}
	mmUserID := *mmUserIDPtr

	flagKey := "mm_reconnect_notice_v1_" + mmUserID
	if existing, _ := config.Mattermost.KVGet(flagKey); len(existing) > 0 {
		return
	}

	dm, appErr := config.Mattermost.GetDirectChannel(mmUserID, config.BotUserID)
	if appErr != nil || dm == nil {
		return
	}
	msg := "Your Confluence connection needs to be refreshed to enable new @-mention DM notifications. " +
		"Please run `/confluence disconnect` and then `/confluence connect` to grant the additional permissions."
	if _, appErr := config.Mattermost.CreatePost(&model.Post{
		UserId: config.BotUserID, ChannelId: dm.Id, Message: msg,
	}); appErr != nil {
		return
	}
	_ = config.Mattermost.KVSet(flagKey, []byte("1"))
}

func mentionEligibleEvent(internal string) bool {
	switch internal {
	case serializer.PageCreatedEvent,
		serializer.PageUpdatedEvent,
		serializer.CommentCreatedEvent,
		serializer.CommentUpdatedEvent:
		return true
	}
	return false
}

func signForgeBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
