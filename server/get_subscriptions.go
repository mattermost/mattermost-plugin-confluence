package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/service"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) handleGetChannelSubscriptions(w http.ResponseWriter, r *http.Request) {
	channelID := r.FormValue("channel_id")
	userID := r.FormValue("user_id")

	if err := p.HasPermissionToManageSubscriptionForMattermostSide(userID, channelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subscriptions, err := service.GetSubscriptionsByChannelID(channelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
