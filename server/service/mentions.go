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
		return
	}

	for _, accountID := range p.AccountIDs {
		if accountID == p.ActorAccountID {
			continue
		}

		mmUserIDPtr, err := store.GetMattermostUserIDFromConfluenceID(p.InstanceID, accountID)
		if err != nil || mmUserIDPtr == nil || *mmUserIDPtr == "" {
			continue
		}
		mmUserID := *mmUserIDPtr

		if !IsMentionNotificationEnabled(mmUserID) {
			continue
		}

		dmChannel, appErr := config.Mattermost.GetDirectChannel(mmUserID, config.BotUserID)
		if appErr != nil || dmChannel == nil {
			config.Mattermost.LogDebug("mention DM: failed to open DM channel", "userID", mmUserID)
			continue
		}

		post := &model.Post{
			UserId:    config.BotUserID,
			ChannelId: dmChannel.Id,
			Message:   buildMentionMessage(p),
		}
		if _, appErr := config.Mattermost.CreatePost(post); appErr != nil {
			config.Mattermost.LogError("mention DM: failed to create post", "userID", mmUserID, "error", appErr.Error())
		}
	}
}

func buildMentionMessage(p MentionDispatchParams) string {
	if p.Kind == ContentKindComment {
		return fmt.Sprintf("You were mentioned in a [comment](%s) on the [%s](%s) page.", p.PageURL, p.PageTitle, p.PageURL)
	}
	return fmt.Sprintf("You were mentioned on the [%s](%s) page.", p.PageTitle, p.PageURL)
}
