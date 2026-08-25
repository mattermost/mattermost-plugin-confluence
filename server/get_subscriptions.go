package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"

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

	// Autocomplete cannot surface an error, so denied users get no suggestions.
	if access := p.checkSubscriptionAccess(mattermostUserID); !access.Allowed {
		p.client.Log.Debug("User does not have access to fetch subscription list", "UserID", mattermostUserID, "reason", access.Reason)
		b, _ := json.Marshal([]model.AutocompleteListItem{})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
		return
	}

	channelID := r.FormValue("channel_id")
	if _, err := p.API.GetChannel(channelID); err != nil {
		p.client.Log.Error("Invalid channel ID. ChannelID: %s. Error: %s", channelID, err.Error())
		http.Error(w, "Invalid channel ID.", http.StatusBadRequest)
		return
	}

	if !p.hasChannelAccess(mattermostUserID, channelID) {
		p.client.Log.Debug("User does not have access to this channel", "UserID", mattermostUserID, "ChannelID", channelID)
		b, _ := json.Marshal([]model.AutocompleteListItem{})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
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
