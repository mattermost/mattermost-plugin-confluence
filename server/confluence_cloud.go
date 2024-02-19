package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

func (p *Plugin) handleConfluenceCloudWebhook(w http.ResponseWriter, r *http.Request) {
	if status, err := verifyHTTPSecret(p.conf.Secret, r.FormValue("secret")); err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	params := mux.Vars(r)
	event := serializer.ConfluenceCloudEventFromJSON(body)
	eventType := params["event"]

	confluenceURL, err := CreateConfluenceURL(event, eventType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	instance, err := p.getInstanceFromURL(confluenceURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	notification := getNotification(p)

	userIDs, err := notification.GetSubscriptionUserIDs(event, eventType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	instanceID := types.ID(instance.GetURL())

	for _, userID := range userIDs {
		resp, err := p.GetPermissionsForCloudEvent(event, instanceID, userID, instance, eventType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if resp == nil {
			http.Error(w, "Don't have the permission to view this event", http.StatusBadRequest)
			return
		}
		notification.SendConfluenceNotifications(event, eventType, p.conf.botUserID, CloudInstance, userID)
	}

	w.Header().Set("Content-Type", "application/json")
	ReturnStatusOK(w)
}

func (p *Plugin) GetPermissionsForCloudEvent(confluenceCloudEvent *serializer.ConfluenceCloudEvent, instanceID types.ID, userID string, instance Instance, eventType string) (*ConfluenceCloudEvent, error) {
	conn, err := p.userStore.LoadConnection(instanceID, types.ID(userID))
	if err != nil {
		p.API.LogError("Error in loading connection.", "Error", err.Error())
		return nil, err
	}

	client, err := instance.GetClient(conn)
	if err != nil {
		p.API.LogError("Error occurred while fetching client.", "Error", err.Error())
		return nil, err
	}

	resp, err := client.(*confluenceCloudClient).GetEventData(confluenceCloudEvent, eventType)
	if err != nil {
		p.API.LogError("Error occurred while fetching event data.", "Error", err.Error())
		return nil, err
	}

	return resp, nil
}

func CreateConfluenceURL(event *serializer.ConfluenceCloudEvent, eventType string) (string, error) {
	var confluenceURL string
	var err error

	if strings.Contains(eventType, Comment) {
		confluenceURL, err = utils.CreateConfluenceURL(event.Comment.Self)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(eventType, Page) {
		confluenceURL, err = utils.CreateConfluenceURL(event.Page.Self)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(eventType, Space) {
		confluenceURL, err = utils.CreateConfluenceURL(event.Space.Self)
		if err != nil {
			return "", err
		}
	}
	return confluenceURL, nil
}
