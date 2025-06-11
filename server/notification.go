package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/thoas/go-funk"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

var eventActions = map[string]string{
	serializer.PageCreatedEvent:  "published",
	serializer.PageUpdatedEvent:  "updated",
	serializer.PageTrashedEvent:  "trashed",
	serializer.PageRestoredEvent: "restored",
	serializer.PageRemovedEvent:  "removed",
}

type notification struct {
	*Plugin
}

func (p *Plugin) getNotification() *notification {
	return &notification{
		p,
	}
}

func (n *notification) SendConfluenceNotifications(event serializer.ConfluenceEventV2, eventType, botUserID string) {
	url := event.GetURL()
	if url == "" {
		return
	}

	spaceKey, pageID := n.extractSpaceKeyAndPageID(event, eventType)
	if spaceKey == "" || pageID == "" {
		return
	}

	post := event.GetNotificationPost(eventType, url, botUserID)
	if post == nil {
		return
	}

	subscriptionChannelIDs := n.getNotificationChannelIDs(url, spaceKey, pageID, eventType)
	for _, channelID := range subscriptionChannelIDs {
		post.ChannelId = channelID
		if _, err := n.API.CreatePost(post); err != nil {
			n.API.LogError("Unable to create Post in Mattermost", "Error", err.Error())
		}
	}
}

func (n *notification) SendGenericWHNotification(event *serializer.ConfluenceServerWebhookPayload, botUserID, url string) {
	eventType := event.Event

	action, exists := eventActions[eventType]
	if !exists {
		n.client.Log.Info("Unsupported Confluence action. Generic notification will not be sent", "event type", eventType)
		return
	}

	post := &model.Post{
		UserId:  botUserID,
		Message: fmt.Sprintf("Someone %s a page on Confluence with the id %d", action, event.Page.ID),
	}

	urlPageIDSubscriptions, err := service.GetSubscriptionsByURLPageID(url, strconv.FormatInt(event.Page.ID, 10))
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for pageID.", event.Page.ID, "Error", err.Error())
		return
	}

	subscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlPageIDSubscriptions, eventType)
	for _, channelID := range subscriptionChannelIDs {
		post.ChannelId = channelID
		if _, err := n.API.CreatePost(post); err != nil {
			n.API.LogError("Unable to create Post in Mattermost", "Error", err.Error())
		}
	}
}

func (n *notification) extractSpaceKeyAndPageID(event serializer.ConfluenceEventV2, eventType string) (string, string) {
	var spaceKey, pageID string

	switch {
	case strings.Contains(eventType, Comment):
		if e, ok := event.(*ConfluenceServerEvent); ok {
			if e.Comment == nil {
				return "", ""
			}

			spaceKey = e.GetCommentSpaceKey()
			pageID = e.GetCommentContainerID()
		}
	case strings.Contains(eventType, Page):
		if e, ok := event.(*ConfluenceServerEvent); ok {
			if e.Page == nil {
				return "", ""
			}

			spaceKey = e.GetPageSpaceKey()
			pageID = event.GetPageID()
		}
	case strings.Contains(eventType, Space):
		spaceKey = event.GetSpaceKey()
		if spaceKey != "" {
			pageID = ""
		}
	}

	return spaceKey, pageID
}

func (n *notification) getNotificationChannelIDs(url, spaceKey, pageID, eventType string) []string {
	urlSpaceKeySubscriptions, err := service.GetSubscriptionsByURLSpaceKey(url, spaceKey)
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for spaceKey", "SpaceKey", spaceKey, "Error", err.Error())
		return nil
	}
	urlPageIDSubscriptions, err := service.GetSubscriptionsByURLPageID(url, pageID)
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for page", "PageID", pageID, "Error", err.Error())
		return nil
	}

	urlPageIDSubscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlPageIDSubscriptions, eventType)
	urlSpaceKeySubscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlSpaceKeySubscriptions, eventType)

	return util.Deduplicate(append(urlSpaceKeySubscriptionChannelIDs, urlPageIDSubscriptionChannelIDs...))
}

func GetURLSubscriptionChannelIDs(urlSubscriptions serializer.StringArrayMap, eventType string) []string {
	var urlSubscriptionChannelIDs []string

	for channelID, events := range urlSubscriptions {
		if funk.Contains(events, eventType) {
			urlSubscriptionChannelIDs = append(urlSubscriptionChannelIDs, channelID)
		}
	}

	return urlSubscriptionChannelIDs
}
