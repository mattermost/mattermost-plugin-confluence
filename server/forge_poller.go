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

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

const (
	forgePollInterval = 30 * time.Second
	forgePollTimeout  = 20 * time.Second
	forgeDrainBatch   = 100
)

// ForgePoller drains events from the Confluence Forge bridge on a ticker.
// One instance per plugin process.
type ForgePoller struct {
	plugin *Plugin
	client *http.Client

	mu      sync.Mutex
	stop    chan struct{}
	stopped chan struct{}
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
	if fp.stop != nil {
		return // already running
	}
	fp.stop = make(chan struct{})
	fp.stopped = make(chan struct{})
	go fp.loop(fp.stop, fp.stopped)
}

func (fp *ForgePoller) Stop() {
	fp.mu.Lock()
	stop, stopped := fp.stop, fp.stopped
	fp.stop, fp.stopped = nil, nil
	fp.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-stopped
}

func (fp *ForgePoller) loop(stop, stopped chan struct{}) {
	defer close(stopped)
	t := time.NewTicker(forgePollInterval)
	defer t.Stop()

	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := fp.drainOnce(context.Background()); err != nil {
				fp.plugin.client.Log.Debug("forge drain failed", "error", err.Error())
			}
		}
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
	return nil
}

func signForgeBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
