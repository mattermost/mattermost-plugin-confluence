package service

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

const (
	subscriptionNotFound = "subscription with name **%s** not found"
)

// DeleteSubscriptionWithDeps deletes a subscription using injected dependencies
func DeleteSubscriptionWithDeps(channelID, alias string, repo SubscriptionRepository, storeService Store) error {
	subs, gErr := repo.GetSubscriptions()
	if gErr != nil {
		return gErr
	}

	if channelSubscriptions, valid := subs.ByChannelID[channelID]; valid {
		if subscription, ok := channelSubscriptions.GetInsensitiveCase(alias); ok {
			aErr := storeService.AtomicModify(store.GetSubscriptionKey(), func(initialBytes []byte) ([]byte, error) {
				subscriptions, err := serializer.SubscriptionsFromJSON(initialBytes)
				if err != nil {
					return nil, err
				}

				if err := subscription.Remove(subscriptions); err != nil {
					return nil, err
				}

				modifiedBytes, marshalErr := json.Marshal(subscriptions)
				if marshalErr != nil {
					return nil, marshalErr
				}

				return modifiedBytes, nil
			})
			return aErr
		}
	}
	return fmt.Errorf(subscriptionNotFound, alias)
}

// DeleteSubscription deletes a subscription using default dependencies
func DeleteSubscription(channelID, alias string) error {
	return DeleteSubscriptionWithDeps(channelID, alias, NewDefaultSubscriptionRepository(), NewDefaultStore())
}
