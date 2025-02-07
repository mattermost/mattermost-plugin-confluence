package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

var confluenceServerWebhook = &Endpoint{
	Path:          "/server/webhook",
	Method:        http.MethodPost,
	Execute:       handleConfluenceServerWebhook,
	RequiresAdmin: false,
}

func handleConfluenceServerWebhook(w http.ResponseWriter, r *http.Request, p *Plugin) {
	config.Mattermost.LogInfo("Received confluence server event.")

	if status, err := verifyHTTPSecret(config.GetConfig().Secret, r.FormValue("secret")); err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	if p.serverVersionGreaterthan9 {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var event *serializer.ConfluenceServerWebhookPayload
		err = json.Unmarshal(body, &event)
		if err != nil {
			config.Mattermost.LogError("Error occurred while unmarshalling Confluence server webhook payload.", "Error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		pluginConfig := config.GetConfig()
		instanceID := types.ID(pluginConfig.ConfluenceURL)

		mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, types.ID(event.UserKey))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		connection, err := store.LoadConnection(instanceID, *mmUserID, p.pluginVersion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		client, err := p.GetServerClient(instanceID, connection)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var spaceKey string
		if strings.Contains(event.Event, Space) {
			spaceKey, err = client.(*confluenceServerClient).GetSpaceKeyFromSpaceID(event.Space.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			event.Space.SpaceKey = spaceKey
		}

		eventData, err := p.GetEventData(event, client)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		eventData.BaseURL = pluginConfig.ConfluenceURL

		notification := p.getNotification()

		notification.SendConfluenceNotifications(eventData, event.Event, p.BotUserID, mmUserID.String())
	} else {
		event := serializer.ConfluenceServerEventFromJSON(r.Body)
		go service.SendConfluenceNotifications(event, event.Event)
	}

	w.Header().Set("Content-Type", "application/json")
	ReturnStatusOK(w)
}

func (p *Plugin) GetEventData(webhookPayload *serializer.ConfluenceServerWebhookPayload, client Client) (*ConfluenceServerEvent, error) {
	eventData, err := client.(*confluenceServerClient).GetEventData(webhookPayload)
	if err != nil {
		p.API.LogError("Error occurred while fetching event data.", "Error", err.Error())
		return nil, err
	}

	return eventData, nil
}
