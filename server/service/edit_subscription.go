package service

import (
	"encoding/json"
	"errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

func EditSubscription(subscription serializer.Subscription) error {
	subs, err := GetSubscriptions()
	if err != nil {
		return errors.New(generalSaveError)
	}
	if err = subscription.ValidateSubscription(&subs); err != nil {
		return err
	}
	key := store.GetSubscriptionKey()
	err = store.AtomicModify(key, func(initialBytes []byte) ([]byte, error) {
		subscriptions, sErr := serializer.SubscriptionsFromJSON(initialBytes)
		if sErr != nil {
			return nil, sErr
		}
		subscription.Edit(subscriptions)
		modifiedBytes, marshalErr := json.Marshal(subscriptions)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return modifiedBytes, nil
	})

	return err
}
