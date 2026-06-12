// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

type ContentKind int

const (
	ContentKindPage ContentKind = iota
	ContentKindComment
)

type MentionDispatchParams struct {
	InstanceID     string
	AccountIDs     []string
	Kind           ContentKind
	PageTitle      string
	PageURL        string
	ActorAccountID string
}

func SendMentionDMs(p MentionDispatchParams) {
	if len(p.AccountIDs) == 0 || p.InstanceID == "" {
		config.Mattermost.LogInfo("mention DM: skipped, empty AccountIDs or InstanceID", "instance_id", p.InstanceID, "count", len(p.AccountIDs))
		return
	}

	seen := make(map[string]struct{}, len(p.AccountIDs))
	for _, accountID := range p.AccountIDs {
		if accountID == "" {
			continue
		}
		if accountID == p.ActorAccountID {
			config.Mattermost.LogDebug("mention DM: skipped actor self-mention", "account_id", accountID)
			continue
		}
		if _, dup := seen[accountID]; dup {
			continue
		}
		seen[accountID] = struct{}{}

		mmUserIDPtr, err := store.GetMattermostUserIDFromConfluenceID(p.InstanceID, accountID)
		if err != nil || mmUserIDPtr == nil || *mmUserIDPtr == "" {
			config.Mattermost.LogInfo("mention DM: no Mattermost user mapped for Confluence account", "instance_id", p.InstanceID, "account_id", accountID, "err", errString(err))
			continue
		}
		mmUserID := *mmUserIDPtr

		if !IsMentionNotificationEnabled(mmUserID) {
			config.Mattermost.LogInfo("mention DM: user has notifications disabled", "user_id", mmUserID, "account_id", accountID)
			continue
		}

		dmChannel, appErr := config.Mattermost.GetDirectChannel(mmUserID, config.BotUserID)
		if appErr != nil || dmChannel == nil {
			config.Mattermost.LogWarn("mention DM: failed to open DM channel", "user_id", mmUserID, "err", appErrString(appErr))
			continue
		}

		post := &model.Post{
			UserId:    config.BotUserID,
			ChannelId: dmChannel.Id,
			Message:   buildMentionMessage(p),
		}
		if _, appErr := config.Mattermost.CreatePost(post); appErr != nil {
			config.Mattermost.LogError("mention DM: failed to create post", "user_id", mmUserID, "error", appErr.Error())
			continue
		}
		config.Mattermost.LogInfo("mention DM: sent", "user_id", mmUserID, "account_id", accountID, "channel_id", dmChannel.Id)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func appErrString(err *model.AppError) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func buildMentionMessage(p MentionDispatchParams) string {
	if p.Kind == ContentKindComment {
		return fmt.Sprintf("You were mentioned in a [comment](%s) on the [%s](%s) page.", p.PageURL, p.PageTitle, p.PageURL)
	}
	return fmt.Sprintf("You were mentioned on the [%s](%s) page.", p.PageTitle, p.PageURL)
}
