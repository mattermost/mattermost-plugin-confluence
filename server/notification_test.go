package main

import (
	"testing"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

func TestGetNotificationsChannelIDs(t *testing.T) {
	for name, val := range map[string]struct {
		baseURL                             string
		spaceKey                            string
		pageID                              string
		event                               string
		userID                              string
		expected                            int
		urlSpaceKeyCombinationSubscriptions serializer.StringStringArrayMap
		urlPageIDCombinationSubscriptions   serializer.StringStringArrayMap
	}{
		"duplicated channel ids": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			userID:   "testuserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},

				"testtesttest1234": {
					"testuserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testtesttest1234": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
			},
			expected: 2,
		},
		"page event": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.PageRemovedEvent,
			userID:   "testuserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentRemovedEvent, serializer.PageRemovedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
			},
			expected: 1,
		},
		"single notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			userID:   "testuserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{},
			expected:                          1,
		},
		"multiple notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentUpdatedEvent,
			userID:   "testuserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttest": {
					"testuserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testtesttest1234": {
					"testuserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},

				"testtesttest1235": {
					"testuserID": {serializer.PageRemovedEvent, serializer.PageCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testtesttest1236": {
					"testuserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{
				"testtesttesttes8": {
					"testUserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testChannelID2": {
					"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			expected: 4,
		},
		"no notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.PageRemovedEvent,
			userID:   "testUserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testChannelID": {
					"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{
				"testChannelID": {
					"testUserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testChannelID2": {
					"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			expected: 0,
		},
		"multiple subscription single notification": {
			baseURL:  "https://test.com",
			spaceKey: "TEST",
			event:    serializer.CommentCreatedEvent,
			userID:   "testUserID",
			urlSpaceKeyCombinationSubscriptions: serializer.StringStringArrayMap{
				"testChannelID": {
					"testUserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
				"testChannelID2": {
					"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			urlPageIDCombinationSubscriptions: serializer.StringStringArrayMap{},
			expected:                          1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()
			mockAPI.On("LogError",
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string"),
				mock.AnythingOfTypeArgument("string")).Return(nil)
			mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)

			monkey.Patch(service.GetSubscriptionsByURLSpaceKey, func(url, spaceKey string) (serializer.StringStringArrayMap, error) {
				return val.urlSpaceKeyCombinationSubscriptions, nil
			})
			monkey.Patch(service.GetSubscriptionsByURLPageID, func(url, pageID string) (serializer.StringStringArrayMap, error) {
				return val.urlPageIDCombinationSubscriptions, nil
			})

			n := getNotification(&Plugin{})
			n.SetAPI(mockAPI)

			channelIDs := n.getNotificationChannelIDs(val.baseURL, val.spaceKey, val.pageID, val.event, val.userID)
			assert.Equal(t, val.expected, len(channelIDs))
		})
	}
}
