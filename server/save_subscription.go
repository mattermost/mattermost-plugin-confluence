package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"

	"github.com/mattermost/mattermost/server/public/model"
)

const subscriptionSaveSuccess = "Your subscription has been saved."

var saveChannelSubscription = &Endpoint{
	Path:            "/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}",
	Method:          http.MethodPost,
	Execute:         handleSaveSubscription,
	IsAuthenticated: true,
}

func handleSaveSubscription(w http.ResponseWriter, r *http.Request, p *Plugin) {
	params := mux.Vars(r)
	channelID := params["channelID"]
	subscriptionType := params["type"]
	userID := r.Header.Get(config.HeaderMattermostUserID)
	var subscription serializer.Subscription

	if !util.IsSystemAdmin(userID) {
		p.client.Log.Error("Non admin user does not have access to create subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "only system admin can save a subscription", http.StatusForbidden)
		return
	}

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to create subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "User does not have access to this channel", http.StatusForbidden)
		return
	}

	var sErr error
	switch subscriptionType {
	case serializer.SubscriptionTypeSpace:
		subscription, sErr = serializer.SpaceSubscriptionFromJSON(r.Body, subscriptionType)
	case serializer.SubscriptionTypePage:
		subscription, sErr = serializer.PageSubscriptionFromJSON(r.Body, subscriptionType)
	default:
		p.client.Log.Error("Invalid subscription type", "Subscription Type", subscriptionType)
		http.Error(w, "Invalid subscription type", http.StatusBadRequest)
		return
	}
	if sErr != nil {
		config.Mattermost.LogError("Error decoding request body.", "Error", sErr.Error())
		http.Error(w, fmt.Sprintf("Could not decode request body. %s", sErr.Error()), http.StatusBadRequest)
		return
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		if statusCode, err := p.validateUserConfluenceAccess(userID, pluginConfig.ConfluenceURL, subscriptionType, subscription); err != nil {
			p.client.Log.Error("Error validating the user's Confluence access", "error", err.Error())
			http.Error(w, err.Error(), statusCode) // safe to return the error string directly, as this function ensures all returned errors are user-friendly
			return
		}
	}

	if err := serializer.ValidateEventsForServerVersion(subscription, pluginConfig.ServerVersionGreaterthan9); err != nil {
		p.client.Log.Error("Invalid events for Confluence Server version", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if statusCode, sErr := service.SaveSubscription(subscription); sErr != nil {
		config.Mattermost.LogError("Error occurred while saving subscription", "Subscription Name", subscription.Name(), "error", sErr.Error())
		http.Error(w, sErr.Error(), statusCode) // safe to return the error string directly, as this function ensures all returned errors are user-friendly
		return
	}

	post := &model.Post{
		UserId:    config.BotUserID,
		ChannelId: channelID,
		Message:   subscriptionSaveSuccess,
	}

	_ = config.Mattermost.SendEphemeralPost(userID, post)
	ReturnStatusOK(w)
}
