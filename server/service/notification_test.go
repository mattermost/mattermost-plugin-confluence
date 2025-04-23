package service

import (
	"testing"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

func baseMock() *plugintest.API {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI

	return mockAPI
}

func TestGetNotificationsChannelIDs(t *testing.T) {
	for name, val := range map[string]struct {
		baseURL                             string
		spaceKey                            string
		pageID                              string
		event                               string
		expected                            int
		urlSpaceKeyCombinationSubscriptions serializer.StringArrayMap
		urlPageIDCombinationSubscriptions   serializer.StringArrayMap
	}{
		"duplicated channel ids": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 1,
		},
		"page event": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.PageRemovedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentRemovedEvent, serializer.PageRemovedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 1,
		},
		"single notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{},
			expected:                          1,
		},
		"multiple notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentUpdatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1235": {serializer.PageRemovedEvent, serializer.PageCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1236": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttes8": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 5,
		},
		"no notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.PageRemovedEvent,
			urlSpaceKeyCombinationSubscriptions: map[string][]string{
				"testtesttesttest": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 0,
		},
		"multiple subscription single notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: map[string][]string{
				"testtesttesttest": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				"testtesttest1234": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{},
			expected:                          1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()
			mockAPI.On("LogError",
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string")).Return(nil)
			mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
			monkey.Patch(GetSubscriptionsByURLSpaceKey, func(_, _ string) (serializer.StringArrayMap, error) {
				return val.urlSpaceKeyCombinationSubscriptions, nil
			})
			monkey.Patch(GetSubscriptionsByURLPageID, func(_, _ string) (serializer.StringArrayMap, error) {
				return val.urlPageIDCombinationSubscriptions, nil
			})
			channelIDs := getNotificationChannelIDs(val.baseURL, val.spaceKey, val.pageID, val.event)
			assert.Equal(t, val.expected, len(channelIDs))
		})
	}
}
