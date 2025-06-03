package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"

	"github.com/mattermost/mattermost/server/public/model"
)

var autocompleteGetChannelSubscriptions = &Endpoint{
	Path:            "/autocomplete/GetChannelSubscriptions",
	Method:          http.MethodGet,
	Execute:         handleGetChannelSubscriptions,
	IsAuthenticated: true,
}

func handleGetChannelSubscriptions(w http.ResponseWriter, r *http.Request, p *Plugin) {
	mattermostUserID := r.Header.Get(config.HeaderMattermostUserID)

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		conn, err := store.LoadConnection(pluginConfig.ConfluenceURL, mattermostUserID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				out := []model.AutocompleteListItem{}
				b, _ := json.Marshal(out)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(b)
				return
			}

			p.client.Log.Error("Error loading connection for the user", "UserID", mattermostUserID, "error", err.Error())
			http.Error(w, "Failed to get subscriptions for the channel", http.StatusInternalServerError)
			return
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			out := []model.AutocompleteListItem{}
			b, _ := json.Marshal(out)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
			return
		}
	}

	channelID := r.FormValue("channel_id")
	if _, err := p.API.GetChannel(channelID); err != nil {
		http.Error(w, "invalid channel ID", http.StatusBadRequest)
		return
	}

	subscriptions, err := service.GetSubscriptionsByChannelID(channelID)
	if err != nil {
		p.client.Log.Error("Error getting subscriptions for the channel", "ChannelID", channelID, "error", err.Error())
		http.Error(w, "Failed to get subscription for the channel", http.StatusInternalServerError)
		return
	}

	out := []model.AutocompleteListItem{}
	for _, sub := range subscriptions {
		out = append(out, model.AutocompleteListItem{
			Item: sub.GetAlias(),
		})
	}

	b, _ := json.Marshal(out)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
