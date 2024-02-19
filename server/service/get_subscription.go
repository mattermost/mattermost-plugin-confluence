package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

const (
	generalError         = "some error occurred. Please try again after some time"
	subscriptionNotFound = "subscription with name **%s** not found"
)

func GetChannelSubscription(channelID, alias string) (serializer.Subscription, int, error) {
	channelSubscriptions, gErr := GetSubscriptionsByChannelID(channelID)
	if gErr != nil {
		return nil, http.StatusInternalServerError, errors.New(generalError)
	}
	subscription, found := channelSubscriptions.GetInsensitiveCase(alias)
	if !found {
		return nil, http.StatusBadRequest, fmt.Errorf(subscriptionNotFound, alias)
	}
	return subscription, http.StatusOK, nil
}
