package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

const (
	createSubscriptionPath    = "/api/v1/instance/%s/%s"
	subscriptionCreatedHeader = "\nThe list of subscriptions created successfully:-\n"
	subscriptionFailedHeader  = "\nThe list of failed subscriptions:-\n"
)

type MigrateSubscriptionBody struct {
	SubscriptionType string   `json:"subscriptionType"`
	Alias            string   `json:"alias"`
	BaseURL          string   `json:"baseURL"`
	SpaceKey         string   `json:"spaceKey"`
	PageID           string   `json:"pageID"`
	ChannelID        string   `json:"channelID"`
	Events           []string `json:"events"`
}

func (p *Plugin) migrateSubscriptions(subscriptions []serializer.Subscription, userID string) string {
	subscriptionCreated := subscriptionCreatedHeader
	failedSubscription := subscriptionFailedHeader
	for _, sub := range subscriptions {
		requestPayload, err := json.Marshal(&MigrateSubscriptionBody{
			SubscriptionType: sub.Name(),
			Alias:            sub.GetAlias(),
			BaseURL:          sub.GetConfluenceURL(),
			SpaceKey:         sub.GetSpaceKeyOrPageID(),
			PageID:           sub.GetSpaceKeyOrPageID(),
			ChannelID:        sub.GetChannelID(),
			Events:           sub.GetEvents(),
		})
		if err != nil {
			p.API.LogError("Unable to marshal request body for subscription", "Subscription", sub, "Error", err.Error())
			failedSubscription = fmt.Sprintf("%s- %s:%s\n", failedSubscription, sub.GetAlias(), err.Error())
			continue
		}

		path := fmt.Sprintf(createSubscriptionPath, base64.StdEncoding.EncodeToString([]byte(sub.GetConfluenceURL())), sub.GetChannelID())
		_, message, err := p.CreateSubscription(requestPayload, sub.GetChannelID(), sub.Name(), userID, path)
		if err != nil {
			p.API.LogError("Unable to migrate subscription", "Subscription", sub.GetAlias(), "Message", message, "Error", err.Error())
			failedSubscription = fmt.Sprintf("%s- %s:%s\n", failedSubscription, sub.GetAlias(), err.Error())
			continue
		}

		subscriptionCreated = fmt.Sprintf("%s- %s\n", subscriptionCreated, sub.GetAlias())
	}
	if subscriptionCreated == subscriptionCreatedHeader {
		subscriptionCreated = ""
	}
	if failedSubscription == subscriptionFailedHeader {
		failedSubscription = ""
	}

	return fmt.Sprintf("%s\n%s", subscriptionCreated, failedSubscription)
}
