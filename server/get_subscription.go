package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

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
	alias := r.FormValue("alias")
	subscription, errCode, err := service.GetChannelSubscription(channelID, alias)
	if err != nil {
		p.client.Log.Error("Error getting subscription for the channel", "ChannelID", channelID, "Subscription Alias", alias, "error", err.Error())
		http.Error(w, "Failed to get subscription for the channel", errCode)
		return
	}

	b, _ := json.Marshal(subscription)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(string(b)))
}
