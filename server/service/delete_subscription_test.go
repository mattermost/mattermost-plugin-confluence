package service

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service/mocks"
)

func TestDeleteSubscription(t *testing.T) {
	subscriptions := serializer.Subscriptions{
		ByChannelID: map[string]serializer.StringSubscription{
			testChannelID1: {
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
			testChannelID2: {
				testAliasPage1: serializer.PageSubscription{
					PageID: testPageID1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasPage1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
		},
		ByURLSpaceKey: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/TEST": {
				testChannelID1: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
		ByURLPageID: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/PAGE-12345": {
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
	}

	for name, val := range map[string]struct {
		channelID string
		alias     string
		apiCalls  func(t *testing.T, channelID, alias string, mockRepo *mocks.MockSubscriptionRepository, mockStore *mocks.MockStore)
	}{
		"space subscription delete success": {
			channelID: testChannelID1,
			alias:     testAliasSpace1,
			apiCalls: func(t *testing.T, channelID, alias string, mockRepo *mocks.MockSubscriptionRepository, mockStore *mocks.MockStore) {
				err := DeleteSubscriptionWithDeps(channelID, alias, mockRepo, mockStore)
				assert.Nil(t, err)
			},
		},
		"page subscription delete success": {
			channelID: testChannelID2,
			alias:     testAliasPage1,
			apiCalls: func(t *testing.T, channelID, alias string, mockRepo *mocks.MockSubscriptionRepository, mockStore *mocks.MockStore) {
				err := DeleteSubscriptionWithDeps(channelID, alias, mockRepo, mockStore)
				assert.Nil(t, err)
			},
		},
		"subscription not found with alias": {
			channelID: testChannelID1,
			alias:     testAliasNotFound,
			apiCalls: func(t *testing.T, channelID, alias string, mockRepo *mocks.MockSubscriptionRepository, mockStore *mocks.MockStore) {
				err := DeleteSubscriptionWithDeps(channelID, alias, mockRepo, mockStore)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
		"no subscription for the channel": {
			channelID: testChannelNotFound,
			alias:     "some-subscription",
			apiCalls: func(t *testing.T, channelID, alias string, mockRepo *mocks.MockSubscriptionRepository, mockStore *mocks.MockStore) {
				err := DeleteSubscriptionWithDeps(channelID, alias, mockRepo, mockStore)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockStore := mocks.NewMockStore(ctrl)

			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()
			mockStore.EXPECT().AtomicModify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			val.apiCalls(t, val.channelID, val.alias, mockRepo, mockStore)
		})
	}
}
