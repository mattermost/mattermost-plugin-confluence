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

	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
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
		fp.plugin.client.Log.Debug("forge drain: skipped, missing config",
			"drain_url_set", cfg.ForgeDrainURL != "",
			"shared_secret_set", cfg.ForgeSharedSecret != "")
		return nil
	}
	fp.plugin.client.Log.Debug("drain: invoked", "url", cfg.ForgeDrainURL)

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
	fp.plugin.client.Log.Debug("forge dispatch", "event", internal, "base_url", forgeEvent.BaseURL, "space_key", forgeEvent.GetSpaceKey(), "page_id", forgeEvent.GetPageID())

	go service.SendConfluenceNotifications(forgeEvent, internal)
	go fp.dispatchMentionDMs(forgeEvent, internal)
	return nil
}

func (fp *ForgePoller) dispatchMentionDMs(evt *serializer.ForgeEvent, internalEvent string) {
	log := fp.plugin.client.Log
	if evt == nil || evt.Content == nil {
		log.Debug("mention DM: skipped, no content on event", "event", internalEvent)
		return
	}
	if !mentionEligibleEvent(internalEvent) {
		log.Debug("mention DM: skipped, event not mention-eligible", "event", internalEvent, "content_id", evt.Content.ID.String())
		return
	}
	if evt.Content.Body == "" {
		log.Debug("mention DM: skipped, event has no body", "event", internalEvent, "content_id", evt.Content.ID.String())
		return
	}

	accountIDs, err := service.ExtractMentionAccountIDsFromADF([]byte(evt.Content.Body))
	if err != nil {
		log.Warn("mention DM: failed to parse ADF body", "content_id", evt.Content.ID.String(), "error", err.Error())
		return
	}
	log.Debug("mention DM: parsed mentions", "event", internalEvent, "content_id", evt.Content.ID.String(), "mention_count", len(accountIDs))
	if len(accountIDs) == 0 {
		return
	}

	kind := service.ContentKindPage
	if evt.Content.Type == "comment" {
		kind = service.ContentKindComment
	}
	pageTitle, pageURL := evt.MentionPageContext()
	instanceID := config.GetConfig().GetConfluenceBaseURL()
	log.Debug("mention DM: dispatching", "instance_id", instanceID, "kind", kind, "page_url", pageURL, "recipients", len(accountIDs))
	service.SendMentionDMs(service.MentionDispatchParams{
		InstanceID:     instanceID,
		AccountIDs:     accountIDs,
		Kind:           kind,
		PageTitle:      pageTitle,
		PageURL:        pageURL,
		ActorAccountID: evt.ActorAccountID(),
	})
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
