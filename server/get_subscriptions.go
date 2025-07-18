package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"

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

	if !util.IsSystemAdmin(mattermostUserID) {
		p.client.Log.Error("Non admin user does not have access to fetch subscription list", "UserID", mattermostUserID)
		http.Error(w, "only system admin can fetch subscription list", http.StatusForbidden)
		return
	}

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

			p.client.Log.Error("Error loading Confluence connection.", "UserID", mattermostUserID, "Error", err.Error())
			http.Error(w, "Unable to fetch user's Confluence connection.", http.StatusInternalServerError)
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
		p.client.Log.Error("Invalid channel ID. ChannelID: %s. Error: %s", channelID, err.Error())
		http.Error(w, "Invalid channel ID.", http.StatusBadRequest)
		return
	}

	subscriptions, err := service.GetSubscriptionsByChannelID(channelID)
	if err != nil {
		p.client.Log.Error("Error retrieving subscriptions. ChannelID: %s. Error: %s", channelID, err.Error())
		http.Error(w, "Failed to get subscriptions for this channel.", http.StatusInternalServerError)
		return
	}

	out := make([]model.AutocompleteListItem, 0, len(subscriptions))
	for _, sub := range subscriptions {
		out = append(out, model.AutocompleteListItem{
			Item: sub.GetAlias(),
		})
	}

	b, _ := json.Marshal(out)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
