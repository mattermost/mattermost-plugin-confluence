// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import "github.com/mattermost/mattermost-plugin-confluence/server/config"

const mentionNotifKeyPrefix = "mm_mention_notif_"

func mentionNotifKey(mattermostUserID string) string {
	return mentionNotifKeyPrefix + mattermostUserID
}

// Default ON: absent KV value means enabled.
func IsMentionNotificationEnabled(mattermostUserID string) bool {
	data, appErr := config.Mattermost.KVGet(mentionNotifKey(mattermostUserID))
	if appErr != nil || len(data) == 0 {
		return true
	}
	return string(data) != "0"
}

func SetMentionNotificationEnabled(mattermostUserID string, enabled bool) error {
	val := []byte("1")
	if !enabled {
		val = []byte("0")
	}
	if appErr := config.Mattermost.KVSet(mentionNotifKey(mattermostUserID), val); appErr != nil {
		return appErr
	}
	return nil
}
