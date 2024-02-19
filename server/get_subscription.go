package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

var GetChannelSubscription = &Endpoint{
	RequiresAdmin: false,
	Path:          "/{channelID:[A-Za-z0-9]+}/subscription",
	Method:        http.MethodGet,
	Execute:       handleGetChannelSubscription,
}

func handleGetChannelSubscription(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params[ParamChannelID]
	alias := r.FormValue("alias")
	subscription, errCode, err := service.GetChannelSubscription(channelID, alias)
	if err != nil {
		http.Error(w, err.Error(), errCode)
		return
	}
	b, _ := json.Marshal(subscription)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(string(b)))
}
