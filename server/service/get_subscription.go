package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

const generalError = "some error occurred. Please try again after some time"

// GetChannelSubscriptionWithDeps gets a channel subscription using injected dependencies
func GetChannelSubscriptionWithDeps(channelID, alias string, repo SubscriptionRepository) (serializer.Subscription, int, error) {
	subs, gErr := repo.GetSubscriptions()
	if gErr != nil {
		return nil, http.StatusInternalServerError, errors.New(generalError)
	}
	channelSubscriptions := subs.ByChannelID[channelID]
	subscription, found := channelSubscriptions.GetInsensitiveCase(alias)
	if !found {
		return nil, http.StatusBadRequest, fmt.Errorf(subscriptionNotFound, alias)
	}
	return subscription, http.StatusOK, nil
}

// GetChannelSubscription gets a channel subscription using default dependencies
func GetChannelSubscription(channelID, alias string) (serializer.Subscription, int, error) {
	return GetChannelSubscriptionWithDeps(channelID, alias, NewDefaultSubscriptionRepository())
}
