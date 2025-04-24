package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

var editChannelSubscription = &Endpoint{
	Path:    "/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}",
	Method:  http.MethodPut,
	Execute: handleEditChannelSubscription,
}

const subscriptionEditSuccess = "Your subscription has been edited successfully."

func handleEditChannelSubscription(w http.ResponseWriter, r *http.Request, _ *Plugin) {
	params := mux.Vars(r)
	channelID := params["channelID"]
	subscriptionType := params["type"]
	userID := r.Header.Get(config.HeaderMattermostUserID)
	var subscription serializer.Subscription
	var err error
	switch subscriptionType {
	case serializer.SubscriptionTypeSpace:
		subscription, err = serializer.SpaceSubscriptionFromJSON(r.Body)
		if err != nil {
			config.Mattermost.LogError("Error decoding request body.", "Error", err.Error())
			http.Error(w, "Could not decode request body", http.StatusBadRequest)
			return
		}
	case serializer.SubscriptionTypePage:
		subscription, err = serializer.PageSubscriptionFromJSON(r.Body)
		if err != nil {
			config.Mattermost.LogError("Error decoding request body.", "Error", err.Error())
			http.Error(w, "Could not decode request body", http.StatusBadRequest)
			return
		}
	}
	if err := service.EditSubscription(subscription); err != nil {
		config.Mattermost.LogError(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
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
