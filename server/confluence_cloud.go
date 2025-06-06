package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

var confluenceCloudWebhook = &Endpoint{
	Path:            "/cloud/{event:[A-Za-z0-9_]+}",
	Method:          http.MethodPost,
	Execute:         handleConfluenceCloudWebhook,
	IsAuthenticated: false,
}

func handleConfluenceCloudWebhook(w http.ResponseWriter, r *http.Request, p *Plugin) {
	p.client.Log.Info("Received Confluence cloud event.")

	if status, err := verifyHTTPSecret(config.GetConfig().Secret, r.FormValue("secret")); err != nil {
		p.client.Log.Error("Error verifying the secret for the Confluence cloud webhook", "error", err.Error())
		http.Error(w, "Failed to verify the secret for the Confluence cloud webhook", status)
		return
	}

	params := mux.Vars(r)
	event, err := serializer.ConfluenceCloudEventFromJSON(r.Body)
	if err != nil {
		p.client.Log.Error("Error occurred while unmarshalling Confluence cloud webhook payload", "error", err)
		http.Error(w, "Failed to process Confluence cloud webhook data", http.StatusInternalServerError)
		return
	}

	go service.SendConfluenceNotifications(event, params["event"])

	w.Header().Set("Content-Type", "application/json")
	ReturnStatusOK(w)
}
