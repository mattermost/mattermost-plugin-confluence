package service

import (
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service/mocks"
)

func TestSaveSpaceSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		newSubscription serializer.SpaceSubscription
		statusCode      int
		errorMessage    string
	}{
		"alias already exist": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: testSpaceKey1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     testAliasSpace1,
					BaseURL:   testBaseURL,
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: aliasAlreadyExist,
		},
		"url space key combination already exist": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: testSpaceKey1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-space-alias",
					BaseURL:   testBaseURL,
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: urlSpaceKeyAlreadyExist,
		},
		"subscription unique base url": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: testSpaceKey1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-space-alias",
					BaseURL:   "https://other.confluence.com",
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription unique space key": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: testSpaceKey2,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-space-alias",
					BaseURL:   testBaseURL,
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subscriptions := getBaseTestSubscriptions()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockStore := mocks.NewMockStore(ctrl)

			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()
			mockStore.EXPECT().AtomicModify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			statusCode, err := SaveSubscriptionWithDeps(val.newSubscription, mockRepo, mockStore)
			assert.Equal(t, val.statusCode, statusCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
			}
		})
	}
}

func TestSavePageSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		newSubscription serializer.PageSubscription
		statusCode      int
		errorMessage    string
	}{
		"alias already exist": {
			newSubscription: serializer.PageSubscription{
				PageID: testPageID1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     testAliasPage1,
					BaseURL:   testBaseURL,
					ChannelID: testChannelID2,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: aliasAlreadyExist,
		},
		"url page id combination already exist": {
			newSubscription: serializer.PageSubscription{
				PageID: testPageID1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-page-alias",
					BaseURL:   testBaseURL,
					ChannelID: testChannelID2,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: urlPageIDAlreadyExist,
		},
		"subscription unique base url": {
			newSubscription: serializer.PageSubscription{
				PageID: testPageID1,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-page-alias",
					BaseURL:   "https://other.confluence.com",
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription unique page id": {
			newSubscription: serializer.PageSubscription{
				PageID: testPageID2,
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "new-page-alias",
					BaseURL:   testBaseURL,
					ChannelID: testChannelID1,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			subscriptions := getBaseTestSubscriptions()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockStore := mocks.NewMockStore(ctrl)

			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()
			mockStore.EXPECT().AtomicModify(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			errCode, err := SaveSubscriptionWithDeps(val.newSubscription, mockRepo, mockStore)
			assert.Equal(t, val.statusCode, errCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
			}
		})
	}
}
