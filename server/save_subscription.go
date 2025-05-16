package main

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"

	"github.com/mattermost/mattermost/server/public/model"
)

const subscriptionSaveSuccess = "Your subscription has been saved."

var saveChannelSubscription = &Endpoint{
	Path:    "/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}",
	Method:  http.MethodPost,
	Execute: handleSaveSubscription,
}

func handleSaveSubscription(w http.ResponseWriter, r *http.Request, p *Plugin) {
	params := mux.Vars(r)
	channelID := params["channelID"]
	subscriptionType := params["type"]
	userID := r.Header.Get(config.HeaderMattermostUserID)
	var subscription serializer.Subscription
	var err error

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to create subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "user does not have access to this channel", http.StatusForbidden)
		return
	}

	switch subscriptionType {
	case serializer.SubscriptionTypeSpace:
		subscription, err = serializer.SpaceSubscriptionFromJSON(r.Body)
	case serializer.SubscriptionTypePage:
		subscription, err = serializer.PageSubscriptionFromJSON(r.Body)
	default:
		p.client.Log.Error("Invalid subscription type", "Subscription Type", subscriptionType)
		http.Error(w, "Invalid subscription type", http.StatusBadRequest)
		return
	}

	if err != nil {
		config.Mattermost.LogError("Error decoding request body.", "Error", err.Error())
		http.Error(w, "Could not decode request body", http.StatusBadRequest)
		return
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		conn, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "User not connected to Confluence", http.StatusUnauthorized)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			http.Error(w, "User not connected to Confluence", http.StatusUnauthorized)
			return
		}
	}

	if statusCode, sErr := service.SaveSubscription(subscription); sErr != nil {
		config.Mattermost.LogError("Error occurred while saving subscription", "Subscription Name", subscription.Name(), "error", sErr.Error())
		http.Error(w, "Failed to save subscription", statusCode)
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
