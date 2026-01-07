package service

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service/mocks"
)

func TestGetChannelSubscription(t *testing.T) {
	subscriptions := getBaseTestSubscriptions()

	for name, val := range map[string]struct {
		channelID    string
		alias        string
		statusCode   int
		errorMessage string
	}{
		"get subscription success": {
			channelID:    testChannelID1,
			alias:        testAliasSpace1,
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription not found for alias": {
			channelID:    testChannelID1,
			alias:        testAliasNotFound,
			statusCode:   http.StatusBadRequest,
			errorMessage: fmt.Sprintf(subscriptionNotFound, testAliasNotFound),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := mocks.NewMockSubscriptionRepository(ctrl)
			mockRepo.EXPECT().GetSubscriptions().Return(subscriptions, nil).AnyTimes()

			subscription, errCode, err := GetChannelSubscriptionWithDeps(val.channelID, val.alias, mockRepo)
			assert.Equal(t, val.statusCode, errCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
				return
			}
			assert.NotNil(t, subscription)
			assert.Equal(t, subscription.(serializer.SpaceSubscription).Alias, val.alias)
		})
	}
}
