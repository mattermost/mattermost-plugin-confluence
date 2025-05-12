package main

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

var editChannelSubscription = &Endpoint{
	Path:    "/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}",
	Method:  http.MethodPut,
	Execute: handleEditChannelSubscription,
}

const subscriptionEditSuccess = "Your subscription has been edited successfully."

func handleEditChannelSubscription(w http.ResponseWriter, r *http.Request, p *Plugin) {
	params := mux.Vars(r)
	channelID := params["channelID"]
	subscriptionType := params["type"]
	userID := r.Header.Get(config.HeaderMattermostUserID)
	var subscription serializer.Subscription
	var err error

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to edit subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "user does not have access to this channel", http.StatusForbidden)
		return
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		var conn *types.Connection
		conn, err = store.LoadConnection(pluginConfig.ConfluenceURL, userID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "User not connected to confluence", http.StatusUnauthorized)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			http.Error(w, "User not connected to confluence", http.StatusUnauthorized)
			return
		}
	}

	switch subscriptionType {
	case serializer.SubscriptionTypeSpace:
		subscription, err = serializer.SpaceSubscriptionFromJSON(r.Body)
	case serializer.SubscriptionTypePage:
		subscription, err = serializer.PageSubscriptionFromJSON(r.Body)
	default:
		p.client.Log.Error("Error updating channel subscription", "Subscription Type", subscriptionType, "error", "Invalid subscription type")
		http.Error(w, "Invalid subscription type", http.StatusBadRequest)
		return
	}

	if err != nil {
		config.Mattermost.LogError("Error decoding request body.", "Error", err.Error())
		http.Error(w, "Could not decode request body", http.StatusBadRequest)
		return
	}

	if nErr := service.EditSubscription(subscription); nErr != nil {
		config.Mattermost.LogError(nErr.Error())
		http.Error(w, nErr.Error(), http.StatusBadRequest)
		return
	}

	post := &model.Post{
		UserId:    config.BotUserID,
		ChannelId: channelID,
		Message:   subscriptionEditSuccess,
	}

	_ = config.Mattermost.SendEphemeralPost(userID, post)
	ReturnStatusOK(w)
}
