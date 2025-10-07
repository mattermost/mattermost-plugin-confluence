package service

import (
	"encoding/json"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

func EditSubscription(subscription serializer.Subscription) error {
	key := store.GetSubscriptionKey()
	err := store.AtomicModify(key, func(initialBytes []byte) ([]byte, error) {
		subscriptions, err := serializer.SubscriptionsFromJSON(initialBytes)
		if err != nil {
			return nil, err
		}

		if err := subscription.Edit(subscriptions); err != nil {
			return nil, err
		}

		modifiedBytes, marshalErr := json.Marshal(subscriptions)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return modifiedBytes, nil
	})

	return err
}
