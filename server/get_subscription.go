package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
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

	if !util.IsSystemAdmin(userID) {
		p.client.Log.Error("Non admin user does not have access to fetch subscription for this channel", "UserID", userID, "ChannelID", channelID)
		http.Error(w, "only system admin can fetch a subscription", http.StatusForbidden)
		return
	}

	if !p.hasChannelAccess(userID, channelID) {
		p.client.Log.Error("User does not have access to get subscription for this channel. UserID: %s, ChannelID: %s", userID, channelID)
		http.Error(w, "User does not have access to this channel.", http.StatusForbidden)
		return
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		conn, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				p.client.Log.Info("User not connected to Confluence. UserID: %s. Error: %s", userID, err.Error())
				http.Error(w, "User not connected to Confluence.", http.StatusUnauthorized)
				return
			}
			p.client.Log.Error("Error loading Confluence connection. UserID: %s. Error: %s", userID, err.Error())
			http.Error(w, "An error occurred while verifying user's Confluence connection.", http.StatusInternalServerError)
			return
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			p.client.Log.Error("User not connected to Confluence. UserID: %s", userID)
			http.Error(w, "User not connected to Confluence.", http.StatusUnauthorized)
			return
		}
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
