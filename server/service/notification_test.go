package service

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service/mocks"
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
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 1,
		},
		"page event": {
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.PageRemovedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentRemovedEvent, serializer.PageRemovedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 1,
		},
		"single notification": {
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{},
			expected:                          1,
		},
		"multiple notification": {
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.CommentUpdatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1:      {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2:      {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				testChannelID3:      {serializer.PageRemovedEvent, serializer.PageCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelNotFound: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 4,
		},
		"no notification": {
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.PageRemovedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			expected: 0,
		},
		"multiple subscription single notification": {
			baseURL:  testBaseURL,
			spaceKey: testSpaceKey1,
			event:    serializer.CommentCreatedEvent,
			urlSpaceKeyCombinationSubscriptions: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			urlPageIDCombinationSubscriptions: serializer.StringArrayMap{},
			expected:                          1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

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
				mock.AnythingOfType("string"),
				mock.AnythingOfType("string")).Return(nil)
			mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockRepo.EXPECT().GetSubscriptionsByURLSpaceKey(gomock.Any(), gomock.Any()).Return(val.urlSpaceKeyCombinationSubscriptions, nil).AnyTimes()
			mockRepo.EXPECT().GetSubscriptionsByURLPageID(gomock.Any(), gomock.Any()).Return(val.urlPageIDCombinationSubscriptions, nil).AnyTimes()

			channelIDs := getNotificationChannelIDsWithDeps(val.baseURL, val.spaceKey, val.pageID, val.event, mockRepo)
			assert.Equal(t, val.expected, len(channelIDs))
		})
	}
}
