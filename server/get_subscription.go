package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

var getChannelSubscription = &Endpoint{
	Path:            "/{channelID:[A-Za-z0-9]+}/subscription",
	Method:          http.MethodGet,
	Execute:         handleGetChannelSubscription,
	IsAuthenticated: true,
}

func handleGetChannelSubscription(w http.ResponseWriter, r *http.Request, p *Plugin) {
	params := mux.Vars(r)
	channelID := params["channelID"]
	userID := r.Header.Get(config.HeaderMattermostUserID)
	alias := r.FormValue("alias")

	if access := p.checkSubscriptionAccess(userID); !access.Allowed {
		p.client.Log.Error("User does not have access to fetch subscription for this channel", "UserID", userID, "ChannelID", channelID, "reason", access.Reason)
		http.Error(w, access.Message, access.StatusCode)
		return
	}

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to get subscription for this channel. UserID: %s, ChannelID: %s", userID, channelID)
		http.Error(w, "User does not have access to this channel.", http.StatusForbidden)
		return
	}

	subscription, errCode, err := service.GetChannelSubscription(channelID, alias)
	if err != nil {
		p.client.Log.Error("Error getting subscription for the channel. ChannelID: %s, Alias: %s. Error: %s", channelID, alias, err.Error())
		http.Error(w, "Failed to get subscription for this channel.", errCode)
		return
	}

	b, _ := json.Marshal(subscription)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(string(b)))
}
