package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
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

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to get subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "user does not have access to this channel", http.StatusForbidden)
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
