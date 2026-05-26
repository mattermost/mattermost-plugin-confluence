// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func kvValueForMMUser(t *testing.T, id string) []byte {
	t.Helper()
	b, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("marshal mm user id: %v", err)
	}
	return b
}

func TestSendMentionDMs_NoopWhenNoAccountIDs(t *testing.T) {
	api := baseMock()
	SendMentionDMs(MentionDispatchParams{InstanceID: "x", Kind: ContentKindPage})
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestSendMentionDMs_SkipsActor(t *testing.T) {
	api := baseMock()
	SendMentionDMs(MentionDispatchParams{
		InstanceID:     "https://example.atlassian.net",
		AccountIDs:     []string{"actor-1"},
		Kind:           ContentKindPage,
		ActorAccountID: "actor-1",
	})
	api.AssertNotCalled(t, "KVGet", mock.Anything)
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestSendMentionDMs_DMsConnectedUser(t *testing.T) {
	api := baseMock()
	config.BotUserID = "bot-user-id"
	defer func() { config.BotUserID = "" }()

	instanceID := "https://example.atlassian.net"

	api.On("KVGet", instanceID+"_acct-target").Return(kvValueForMMUser(t, "mm-user-1"), (*model.AppError)(nil))
	api.On("KVGet", mentionNotifKey("mm-user-1")).Return(([]byte)(nil), (*model.AppError)(nil))
	api.On("GetDirectChannel", "mm-user-1", "bot-user-id").
		Return(&model.Channel{Id: "dm-channel"}, (*model.AppError)(nil))
	api.On("CreatePost", mock.MatchedBy(func(p *model.Post) bool {
		return p != nil && p.ChannelId == "dm-channel" && p.UserId == "bot-user-id"
	})).Return(&model.Post{}, (*model.AppError)(nil))

	SendMentionDMs(MentionDispatchParams{
		InstanceID:     instanceID,
		AccountIDs:     []string{"acct-target"},
		Kind:           ContentKindPage,
		PageTitle:      "Test Page",
		PageURL:        "https://example.atlassian.net/spaces/AAA/pages/p1",
		ActorAccountID: "acct-author",
	})

	api.AssertCalled(t, "CreatePost", mock.Anything)
}

func TestSendMentionDMs_RespectsUserOptOut(t *testing.T) {
	api := baseMock()
	config.BotUserID = "bot-user-id"
	defer func() { config.BotUserID = "" }()

	instanceID := "https://example.atlassian.net"

	api.On("KVGet", instanceID+"_acct-target").Return(kvValueForMMUser(t, "mm-user-1"), (*model.AppError)(nil))
	api.On("KVGet", mentionNotifKey("mm-user-1")).Return([]byte("0"), (*model.AppError)(nil))

	SendMentionDMs(MentionDispatchParams{
		InstanceID:     instanceID,
		AccountIDs:     []string{"acct-target"},
		Kind:           ContentKindPage,
		ActorAccountID: "acct-author",
	})

	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestBuildMentionMessage(t *testing.T) {
	page := MentionDispatchParams{Kind: ContentKindPage, PageTitle: "Hi", PageURL: "https://x/p/1"}
	comment := MentionDispatchParams{Kind: ContentKindComment, PageTitle: "Hi", PageURL: "https://x/p/1"}

	assert.Contains(t, buildMentionMessage(page), "mentioned on the [Hi](https://x/p/1) page")
	assert.Contains(t, buildMentionMessage(comment), "mentioned in a [comment]")
}
