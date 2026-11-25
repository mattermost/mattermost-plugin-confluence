package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

const (
	generalEditError     = "an error occurred attempting to edit a subscription"
	subscriptionModified = "the subscription was modified by someone else. Please try again"
)

// EditSubscriptionWithDeps replaces the subscription stored under oldAlias in channelID using injected dependencies.
// The stored subscription must still have the creator recorded on the edited subscription.
func EditSubscriptionWithDeps(channelID, oldAlias string, subscription serializer.Subscription, storeService Store) (int, error) {
	statusCode := http.StatusInternalServerError
	var userErr error
	key := store.GetSubscriptionKey()
	err := storeService.AtomicModify(key, func(initialBytes []byte) ([]byte, error) {
		statusCode, userErr = http.StatusInternalServerError, nil
		subscriptions, err := serializer.SubscriptionsFromJSON(initialBytes)
		if err != nil {
			return nil, err
		}

		existing, ok := subscriptions.ByChannelID[channelID][oldAlias]
		if !ok {
			statusCode, userErr = http.StatusBadRequest, fmt.Errorf(subscriptionNotFound, oldAlias)
			return nil, userErr
		}
		if existing.GetCreatedBy() != subscription.GetCreatedBy() {
			statusCode, userErr = http.StatusConflict, errors.New(subscriptionModified)
			return nil, userErr
		}
		if vErr := subscription.ValidateEdit(subscriptions, existing); vErr != nil {
			statusCode, userErr = http.StatusBadRequest, vErr
			return nil, userErr
		}

		if err := subscription.Edit(subscriptions, existing); err != nil {
			return nil, err
		}

		modifiedBytes, marshalErr := json.Marshal(subscriptions)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return modifiedBytes, nil
	})
	if err != nil {
		if userErr != nil {
			return statusCode, userErr
		}
		config.Mattermost.LogError("Error editing subscription in store.", "error", err.Error())
		return http.StatusInternalServerError, errors.New(generalEditError)
	}
	return http.StatusOK, nil
}

// EditSubscription edits a subscription using default dependencies
func EditSubscription(channelID, oldAlias string, subscription serializer.Subscription) (int, error) {
	return EditSubscriptionWithDeps(channelID, oldAlias, subscription, NewDefaultStore())
}
