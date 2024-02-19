package service

import (
	"net/url"
	"strings"

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

func GetSubscriptionsByChannelID(channelID string) (serializer.StringSubscription, error) {
	subscriptions, err := GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByChannelID[channelID], nil
}

func GetSubscriptionsByURLSpaceKey(url, spaceKey string) (serializer.StringStringArrayMap, error) {
	key := store.GetURLSpaceKeyCombinationKey(url, spaceKey)
	subscriptions, err := GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByURLSpaceKey[key], nil
}

func GetSubscriptionsByURLPageID(url, pageID string) (serializer.StringStringArrayMap, error) {
	key := store.GetURLPageIDCombinationKey(url, pageID)
	subscriptions, err := GetSubscriptions()
	if err != nil {
		return nil, err
	}
	return subscriptions.ByURLPageID[key], nil
}

func GetSubscriptionBySpaceID(spaceID string) (string, error) {
	subscriptions, err := GetSubscriptions()
	if err != nil {
		return "", err
	}

	return subscriptions.BySpaceID[spaceID], nil
}

func GetSubscriptionFromURL(confluenceURL, userID string) (int, error) {
	parsedURL, err := url.Parse(confluenceURL)
	if err != nil {
		return 0, err
	}
	subscriptions, err := GetSubscriptions()
	if err != nil {
		return 0, err
	}
	totalSubscriptions := GetTotalSubscriptionFromURL(subscriptions.ByURLPageID, userID, parsedURL) + GetTotalSubscriptionFromURL(subscriptions.ByURLSpaceKey, userID, parsedURL)

	return totalSubscriptions, nil
}

func GetTotalSubscriptionFromURL(subscriptionsMap map[string]serializer.StringStringArrayMap, userID string, url *url.URL) int {
	totalSubscriptions := 0
	for key, channelIDEventsMap := range subscriptionsMap {
		if strings.Contains(key, url.Hostname()) {
			for _, userIDEventsMap := range channelIDEventsMap {
				for id := range userIDEventsMap {
					if id == userID {
						totalSubscriptions++
					}
				}
			}
		}
	}
	return totalSubscriptions
}

func GetOldSubscriptions() ([]serializer.Subscription, error) {
	key := store.GetOldSubscriptionKey()
	initialBytes, appErr := config.Mattermost.KVGet(key)
	if appErr != nil {
		return nil, errors.New(getChannelSubscriptionsError)
	}

	subscriptions, err := serializer.OldSubscriptionsFromJSON(initialBytes)
	if err != nil {
		return nil, errors.New(getChannelSubscriptionsError)
	}

	var subscriptionList []serializer.Subscription
	for _, userIDSubscriptionMap := range subscriptions.ByChannelID {
		for _, subscription := range userIDSubscriptionMap {
			subscriptionList = append(subscriptionList, subscription)
		}
	}

	return subscriptionList, nil
}
