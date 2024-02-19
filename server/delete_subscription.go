package main

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	storePackage "github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const (
	subscriptionNotFound = "subscription with name **%s** not found"
)

func (p *Plugin) DeleteSubscription(channelID, alias, userID string) error {
	subs, err := service.GetSubscriptions()
	if err != nil {
		return err
	}

	channelSubscriptions, isValid := subs.ByChannelID[channelID]
	if !isValid {
		return fmt.Errorf(subscriptionNotFound, alias)
	}

	subscription, ok := channelSubscriptions.GetInsensitiveCase(alias)
	if !ok {
		return fmt.Errorf(subscriptionNotFound, alias)
	}
	instance, err := p.getInstanceFromURL(subscription.GetConfluenceURL())
	if err != nil {
		return err
	}
	if instance.Common().Type == ServerInstanceType {
		totalSubscriptions, err := service.GetSubscriptionFromURL(subscription.GetConfluenceURL(), subscription.GetUserID())
		if err != nil {
			return err
		}

		if totalSubscriptions == 1 {
			adminConn, err := p.userStore.LoadConnection(types.ID(instance.GetURL()), types.ID(AdminMattermostUserID))
			if err != nil {
				return err
			}
			err = p.HasPermissionToManageSubscription(instance.GetURL(), userID, channelID)
			if err != nil {
				return err
			}

			adminClient, err := instance.GetClient(adminConn)
			if err != nil {
				return err
			}

			webhookID, err := p.userStore.LoadWebhookID(types.ID(instance.GetURL()), types.ID(subscription.GetUserID()))
			if err != nil {
				return err
			}

			err = adminClient.(*confluenceServerClient).DeleteWebhook(webhookID)
			if err != nil {
				return err
			}
			err = p.userStore.DeleteWebhookID(types.ID(instance.GetURL()), types.ID(subscription.GetUserID()))
			if err != nil {
				return err
			}
		}
	}

	aErr := storePackage.AtomicModify(storePackage.GetSubscriptionKey(), func(initialBytes []byte) ([]byte, error) {
		subscriptions, err := serializer.SubscriptionsFromJSON(initialBytes)
		if err != nil {
			return nil, err
		}
		subscription.Remove(subscriptions)
		modifiedBytes, marshalErr := json.Marshal(subscriptions)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return modifiedBytes, nil
	})
	return aErr
}
