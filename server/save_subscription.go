package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"

	"github.com/mattermost/mattermost/server/public/model"
)

const subscriptionSaveSuccess = "Your subscription has been saved."

var saveChannelSubscription = &Endpoint{
	Path:            "/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}",
	Method:          http.MethodPost,
	Execute:         handleSaveSubscription,
	IsAuthenticated: true,
}

func handleSaveSubscription(w http.ResponseWriter, r *http.Request, _ *Plugin) {
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
