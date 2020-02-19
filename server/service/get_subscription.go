package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Brightscout/mattermost-plugin-confluence/server/serializer"
)

const generalError = "Some error occurred. Please try again after sometime."

func GetChannelSubscription(channelID, alias string) (serializer.Subscription, int, error) {
	channelSubscriptions, gErr := GetSubscriptionsByChannelID(channelID)
	if gErr != nil {
		return nil, http.StatusInternalServerError, errors.New(generalError)
	}
	subscription, found := channelSubscriptions[alias]
	if !found {
		return nil, http.StatusBadRequest, errors.New(fmt.Sprintf(subscriptionNotFound, alias))
	}
	return subscription, http.StatusOK, nil
}
