package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service/mocks"
)

func TestGetSubscriptionsByChannelID(t *testing.T) {
	for name, val := range map[string]struct {
		channelID string
		expected  serializer.StringSubscription
	}{
		"single subscription": {
			channelID: testChannelID1,
			expected: serializer.StringSubscription{
				testAliasSpace1: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID1,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
		},
		"multiple subscription": {
			channelID: testChannelID2,
			expected: serializer.StringSubscription{
				testAliasPage1: serializer.PageSubscription{
					PageID: testPageID1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasPage1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
				testAliasSpace2: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace2,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
		},
		"no subscription": {
			channelID: testChannelNotFound,
			expected:  serializer.StringSubscription(nil),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subscriptions := getBaseTestSubscriptions()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()

			sub, err := GetSubscriptionsByChannelIDWithDeps(val.channelID, mockRepo)
			assert.Nil(t, err)
			assert.Equal(t, val.expected, sub)
		})
	}
}

func TestGetSubscriptionsByURLPageID(t *testing.T) {
	for name, val := range map[string]struct {
		url      string
		pageID   string
		expected serializer.StringArrayMap
	}{
		"single subscription": {
			url:    testBaseURL,
			pageID: testPageID1,
			expected: serializer.StringArrayMap{
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
		"multiple subscription": {
			url:    testBaseURL,
			pageID: testPageID2,
			expected: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
		"no subscription": {
			url:      testBaseURL,
			pageID:   testPageIDNotFound,
			expected: serializer.StringArrayMap(nil),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subscriptions := getExtendedTestSubscriptions()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()

			sub, err := GetSubscriptionsByURLPageIDWithDeps(val.url, val.pageID, mockRepo)
			assert.Nil(t, err)
			assert.Equal(t, val.expected, sub)
		})
	}
}

func TestGetSubscriptionsByURLSpaceKey(t *testing.T) {
	for name, val := range map[string]struct {
		url      string
		spaceKey string
		expected serializer.StringArrayMap
	}{
		"single subscription": {
			url:      testBaseURL,
			spaceKey: testSpaceKey2,
			expected: serializer.StringArrayMap{
				testChannelID3: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
		"multiple subscription": {
			url:      testBaseURL,
			spaceKey: testSpaceKey1,
			expected: serializer.StringArrayMap{
				testChannelID1: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
		"no subscription": {
			url:      testBaseURL,
			spaceKey: "NOTEXIST",
			expected: serializer.StringArrayMap(nil),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subscriptions := getExtendedTestSubscriptions()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()

			sub, err := GetSubscriptionsByURLSpaceKeyWithDeps(val.url, val.spaceKey, mockRepo)
			assert.Nil(t, err)
			assert.Equal(t, val.expected, sub)
		})
	}
}
