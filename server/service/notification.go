package service

import (
	"slices"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

func SendConfluenceNotifications(event serializer.ConfluenceEvent, eventType string) {
	url := event.GetURL()
	spaceKey := event.GetSpaceKey()
	pageID := event.GetPageID()
	post := event.GetNotificationPost(eventType)

	if post == nil || pageID == "" || url == "" || spaceKey == "" {
		return
	}

	subscriptionChannelIDs := getNotificationChannelIDs(url, spaceKey, pageID, eventType)
	for _, channelID := range subscriptionChannelIDs {
		post.ChannelId = channelID
		if _, err := config.Mattermost.CreatePost(post); err != nil {
			config.Mattermost.LogError("Unable to create Post in Mattermost", "Error", err.Error())
		}
	}
}

func getNotificationChannelIDs(url, spaceKey, pageID, eventType string) []string {
	urlSpaceKeySubscriptions, err := GetSubscriptionsByURLSpaceKey(url, spaceKey)
	if err != nil {
		config.Mattermost.LogError("Unable to get subscribed channels.", "Error", err.Error())
		return nil
	}
	urlPageIDSubscriptions, err := GetSubscriptionsByURLPageID(url, pageID)
	if err != nil {
		config.Mattermost.LogError("Unable to get subscribed channels.", "Error", err.Error())
		return nil
	}

	urlSpaceKeySubscriptionChannelIDs := make([]string, 0)
	urlPageIDSubscriptionChannelIDs := make([]string, 0)
	for channelID, events := range urlPageIDSubscriptions {
		if slices.Contains(events, eventType) {
			urlPageIDSubscriptionChannelIDs = append(urlPageIDSubscriptionChannelIDs, channelID)
		}
	}
	for channelID, events := range urlSpaceKeySubscriptions {
		if slices.Contains(events, eventType) {
			urlSpaceKeySubscriptionChannelIDs = append(urlSpaceKeySubscriptionChannelIDs, channelID)
		}
	}

	var channelIDs []string
	channelIDs = append(channelIDs, urlSpaceKeySubscriptionChannelIDs...)
	channelIDs = append(channelIDs, urlPageIDSubscriptionChannelIDs...)

	return util.Deduplicate(channelIDs)
}
