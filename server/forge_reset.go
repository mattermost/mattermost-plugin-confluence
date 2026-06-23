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
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

const (
	forgeResetTimeout = 20 * time.Second

	forgeResetSuccessMsg = "Forge bridge secret rotated successfully. Queued events cleared: %d. Polling will resume within ~30s."

	forgeResetMissingURLsMsg = "This plugin does not have the Forge reset URL on file. " +
		"Make sure the Forge app has been redeployed (so the `reset` webtrigger exists) " +
		"and run `/confluence install cloud` once to capture its URL. " +
		"Subsequent rotations will then run inline via this command."
)

func executeForgeReset(p *Plugin, ctx *model.CommandArgs, _ ...string) *model.CommandResponse {
	if !util.IsSystemAdmin(ctx.UserId) {
		postCommandResponse(ctx, commandsOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	cfg := config.GetConfig()
	resetURL := strings.TrimSpace(cfg.ForgeResetURL)
	registerURL := strings.TrimSpace(cfg.ForgeRegisterURL)
	currentSecret := strings.TrimSpace(cfg.ForgeSharedSecret)

	if resetURL == "" || registerURL == "" {
		postCommandResponse(ctx, forgeResetMissingURLsMsg)
		return &model.CommandResponse{}
	}
	if currentSecret == "" {
		postCommandResponse(ctx, "Forge Bridge Shared Secret is not set on this plugin; reload the plugin to regenerate it.")
		return &model.CommandResponse{}
	}

	queuedDeleted, err := postForgeReset(resetURL, currentSecret)
	if err != nil {
		p.client.Log.Warn("forge reset: bridge wipe failed", "error", err.Error())
		postCommandResponse(ctx, "Forge bridge reset failed: "+err.Error())
		return &model.CommandResponse{}
	}

	newSecret, err := generateRandomKey(32)
	if err != nil {
		postCommandResponse(ctx, "Failed to generate a new shared secret: "+err.Error())
		return &model.CommandResponse{}
	}

	urls, err := postForgeRegister(registerURL, newSecret)
	if err != nil {
		p.client.Log.Error("forge reset: bridge wiped but re-register failed", "error", err.Error())
		postCommandResponse(ctx,
			"Forge bridge was wiped, but re-registering the new secret failed: "+err.Error()+
				"\n\nThe bridge currently has no shared secret. Run `/confluence install cloud` to re-register, or retry `/confluence forge reset`.")
		return &model.CommandResponse{}
	}

	cfg.ForgeSharedSecret = newSecret
	if urls != nil {
		if isForgeWebtriggerURL(urls.Reset) {
			cfg.ForgeResetURL = urls.Reset
		}
		if isForgeWebtriggerURL(urls.Drain) {
			cfg.ForgeDrainURL = urls.Drain
		}
		if isForgeWebtriggerURL(urls.Register) {
			cfg.ForgeRegisterURL = urls.Register
		}
	}
	cfg.Sanitize()
	configMap, err := cfg.ToMap()
	if err != nil {
		postCommandResponse(ctx, "Secret rotated on the bridge, but failed to serialize plugin config: "+err.Error()+". Re-run `/confluence install cloud` to recover.")
		return &model.CommandResponse{}
	}
	if err := p.client.Configuration.SavePluginConfig(configMap); err != nil {
		postCommandResponse(ctx, "Secret rotated on the bridge, but failed to save plugin config: "+err.Error()+". Re-run `/confluence install cloud` to recover.")
		return &model.CommandResponse{}
	}

	p.client.Log.Info("forge reset: rotated bridge secret", "queued_deleted", queuedDeleted, "user_id", ctx.UserId)
	postCommandResponse(ctx, fmt.Sprintf(forgeResetSuccessMsg, queuedDeleted))
	return &model.CommandResponse{}
}

type forgeResetResponse struct {
	OK            bool `json:"ok"`
	QueuedDeleted int  `json:"queuedDeleted"`
	AlreadyClear  bool `json:"alreadyClear"`
}

func postForgeReset(resetURL, secret string) (int, error) {
	body := []byte{}
	signature := hmacHexSHA256(secret, body)

	ctx, cancel := context.WithTimeout(context.Background(), forgeResetTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resetURL, bytes.NewReader(body))
	if err != nil {
		return 0, errors.Wrap(err, "failed to build reset request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MM-Signature", signature)

	client := &http.Client{
		Timeout: forgeResetTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to reach reset URL")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK:
		var parsed forgeResetResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return 0, errors.Wrap(err, "bridge returned 200 with unparseable body; reset not confirmed")
		}
		if !parsed.OK {
			return 0, errors.New("bridge returned 200 but ok=false; reset not confirmed")
		}
		if parsed.QueuedDeleted < 0 {
			return 0, errors.Errorf("bridge returned invalid queuedDeleted=%d", parsed.QueuedDeleted)
		}
		return parsed.QueuedDeleted, nil
	case http.StatusForbidden:
		return 0, errors.New("bridge rejected reset signature: the plugin's shared secret no longer matches the bridge. Have a Forge admin run `forge invoke -f wipeRegistrationFn -e <env>`, then re-run `/confluence install cloud`")
	default:
		snippet := strings.TrimSpace(string(respBody))
		if snippet == "" {
			snippet = resp.Status
		}
		return 0, errors.Errorf("bridge rejected reset (%d): %s", resp.StatusCode, snippet)
	}
}

func hmacHexSHA256(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
