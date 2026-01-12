package service

import (
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

const getChannelSubscriptionsError = " Error getting channel subscriptions."

func GetSubscriptions() (serializer.Subscriptions, error) {
	key := store.GetSubscriptionKey()
	initialBytes, appErr := config.Mattermost.KVGet(key)
	if appErr != nil {
		return serializer.Subscriptions{}, errors.New(getChannelSubscriptionsError)
	}
	subscriptions, err := serializer.SubscriptionsFromJSON(initialBytes)
	if err != nil {
		return serializer.Subscriptions{}, errors.New(getChannelSubscriptionsError)
	}
	return *subscriptions, nil
}

func GetSubscriptionsByChannelIDWithDeps(channelID string, repo SubscriptionRepository) (serializer.StringSubscription, error) {
	subscriptions, err := repo.GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByChannelID[channelID], nil
}

func GetSubscriptionsByChannelID(channelID string) (serializer.StringSubscription, error) {
	return GetSubscriptionsByChannelIDWithDeps(channelID, NewDefaultSubscriptionRepository())
}

func GetSubscriptionsByURLSpaceKeyWithDeps(url, spaceKey string, repo SubscriptionRepository) (serializer.StringArrayMap, error) {
	key := store.GetURLSpaceKeyCombinationKey(url, spaceKey)
	subscriptions, err := repo.GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByURLSpaceKey[key], nil
}

func GetSubscriptionsByURLSpaceKey(url, spaceKey string) (serializer.StringArrayMap, error) {
	return GetSubscriptionsByURLSpaceKeyWithDeps(url, spaceKey, NewDefaultSubscriptionRepository())
}

func GetSubscriptionsByURLPageIDWithDeps(url, pageID string, repo SubscriptionRepository) (serializer.StringArrayMap, error) {
	key := store.GetURLPageIDCombinationKey(url, pageID)
	subscriptions, err := repo.GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByURLPageID[key], nil
}

func GetSubscriptionsByURLPageID(url, pageID string) (serializer.StringArrayMap, error) {
	return GetSubscriptionsByURLPageIDWithDeps(url, pageID, NewDefaultSubscriptionRepository())
}
