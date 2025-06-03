package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

var confluenceServerWebhook = &Endpoint{
	Path:            "/server/webhook",
	Method:          http.MethodPost,
	Execute:         handleConfluenceServerWebhook,
	IsAuthenticated: false,
}

func handleConfluenceServerWebhook(w http.ResponseWriter, r *http.Request, p *Plugin) {
	p.client.Log.Info("Received Confluence server event.")

	if status, err := verifyHTTPSecret(config.GetConfig().Secret, r.FormValue("secret")); err != nil {
		p.client.Log.Error("Error verifying secret for the Confluence server webhook", "error", err.Error())
		http.Error(w, "Failed to verify secret for the Confluence server webhook", status)
		return
	}

	pluginConfig := config.GetConfig()

	if pluginConfig.ServerVersionGreaterthan9 {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			p.client.Log.Error("Error reading body of the Confluence server webhook", "error", err.Error())
			http.Error(w, "Failed to read body for the Confluence server webhook", http.StatusBadRequest)
			return
		}

		if respondToTestConnection(body) {
			w.Header().Set("Content-Type", "application/json")
			ReturnStatusOK(w)
			return
		}

		var event *serializer.ConfluenceServerWebhookPayload
		err = json.Unmarshal(body, &event)
		if err != nil {
			p.client.Log.Error("Error occurred while unmarshaling Confluence server webhook payload", "Error", err.Error())
			http.Error(w, "Failed to unmarshal Confluence server webhook payload", http.StatusInternalServerError)
			return
		}

		pluginConfig := config.GetConfig()
		instanceID := pluginConfig.ConfluenceURL

		notification := p.getNotification()

		client, _, err := p.GetClientFromUserKey(instanceID, event.UserKey)
		// If there is an error while retrieving the client from the event user key, it could be due to one of the following reasons:
		// - An expected error occurred.
		// - The user who triggered the event in Confluence is not connected to Mattermost.
		// If the Admin API token is available, we will attempt to fetch additional data using it to send a detailed notification.
		// Otherwise, a generic notification will be sent.
		if err != nil {
			if pluginConfig.AdminAPIToken != "" {
				p.client.Log.Info("Error getting client for the user who triggered webhook event. Sending notification using admin API token")
				if strings.Contains(event.Event, Space) {
					var spaceKey string
					spaceKey, err = p.GetSpaceKeyFromSpaceIDWithAPIToken(event.Space.ID, pluginConfig)
					if err != nil {
						p.client.Log.Error("Error getting space key using space ID with API token", "error", err)
						http.Error(w, "Failed to send Confluence notification using API Token", http.StatusInternalServerError)
						return
					}
					event.Space.SpaceKey = spaceKey
				}

				var eventData *ConfluenceServerEvent
				eventData, err = p.GetEventDataWithAPIToken(event, pluginConfig)
				if err != nil {
					p.client.Log.Error("Error getting event data with API token", "error", err)
					http.Error(w, "Failed to send Confluence notification using API Token", http.StatusInternalServerError)
					return
				}

				eventData.BaseURL = pluginConfig.ConfluenceURL
				notification.SendConfluenceNotifications(eventData, event.Event, p.BotUserID)
			} else {
				p.client.Log.Info("Error getting client for the user who triggered webhook event. Sending generic notification")
				notification.SendGenericWHNotification(event, p.BotUserID, pluginConfig.ConfluenceURL)
			}

			w.Header().Set("Content-Type", "application/json")
			ReturnStatusOK(w)
			return
		}

		var spaceKey string
		if strings.Contains(event.Event, Space) {
			spaceKey, err = client.(*confluenceServerClient).GetSpaceKeyFromSpaceID(event.Space.ID)
			if err != nil {
				p.client.Log.Error("Failed to get Space Key from the Space ID", "Space ID", event.Space.ID, "error", err.Error())
				http.Error(w, "Failed to send notification for Confluence server webhook", http.StatusInternalServerError)
				return
			}
			event.Space.SpaceKey = spaceKey
		}

		eventData, err := p.GetEventData(event, client)
		if err != nil {
			p.client.Log.Error("Error getting event data for the Confluence server webhook", "error", err.Error())
			http.Error(w, "Failed to send notification for Confluence server webhook", http.StatusInternalServerError)
			return
		}

		eventData.BaseURL = pluginConfig.ConfluenceURL

		notification.SendConfluenceNotifications(eventData, event.Event, p.BotUserID)
	} else {
		event, err := serializer.ConfluenceServerEventFromJSON(r.Body)
		if err != nil {
			p.client.Log.Error("Error occurred while unmarshalling Confluence server webhook payload", "error", err)
			http.Error(w, "Failed to unmarshal Confluence server webhook payload", http.StatusInternalServerError)
			return
		}

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

func (p *Plugin) GetClientFromUserKey(instanceID, eventUserKey string) (Client, *string, error) {
	mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, eventUserKey)
	if err != nil {
		p.client.Log.Error("Error getting Mattermost User ID from Confluence ID", "InstanceID", instanceID, "Confluence Account ID", eventUserKey, "error", err.Error())
		return nil, nil, err
	}

	connection, err := store.LoadConnection(instanceID, *mmUserID)
	if err != nil {
		p.client.Log.Error("Error loading the connection", "UserID", *mmUserID, "InstanceURL", instanceID, "error", err.Error())
		return nil, nil, err
	}

	client, err := p.GetServerClient(instanceID, connection)
	if err != nil {
		p.client.Log.Error("Error getting server client", "InstanceID", instanceID, "error", err.Error())
		return nil, nil, err
	}

	return client, mmUserID, nil
}

func (p *Plugin) GetSpaceKeyFromSpaceIDWithAPIToken(spaceID int64, pluginConfig *config.Configuration) (string, error) {
	start := 0

	for {
		path := fmt.Sprintf("%s%s?start=%d&limit=%d", pluginConfig.ConfluenceURL, PathSpaceData, start, pageSize)

		response := &apiResponse{}

		body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
		if err != nil || statusCode != http.StatusOK {
			return "", errors.Wrapf(err, "error getting spaceKey from spaceID")
		}

		if err = json.Unmarshal(body, response); err != nil {
			return "", errors.Wrapf(err, "failed to unmarshal spaceKey data")
		}

		for _, space := range response.Results {
			if space.ID == spaceID {
				return space.Key, nil
			}
		}

		if len(response.Results) < pageSize {
			break
		}

		start += pageSize
	}

	return "", fmt.Errorf("confluence GetSpaceKeyFromSpaceIDUsingAPIToken: no space found for the space key")
}

func (p *Plugin) GetEventDataWithAPIToken(webhookPayload *serializer.ConfluenceServerWebhookPayload, pluginConfig *config.Configuration) (*ConfluenceServerEvent, error) {
	var confluenceServerEvent ConfluenceServerEvent
	var err error
	supportedWHEventFound := false

	if strings.Contains(webhookPayload.Event, Comment) {
		supportedWHEventFound = true
		confluenceServerEvent.Comment, err = p.GetCommentDataWithAPIToken(webhookPayload, pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting comment data for the event using API token")
		}
	}

	if strings.Contains(webhookPayload.Event, Page) {
		supportedWHEventFound = true
		confluenceServerEvent.Page, err = p.GetPageDataWithAPIToken(int(webhookPayload.Page.ID), pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting page data for the event using API token")
		}
	}

	if strings.Contains(webhookPayload.Event, Space) {
		supportedWHEventFound = true
		confluenceServerEvent.Space, err = p.GetSpaceDataWithAPIToken(webhookPayload.Space.SpaceKey, pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting space data for the event using API token")
		}
	}

	if !supportedWHEventFound {
		return nil, errors.New("unable to get data for unsupported webhook event")
	}

	return &confluenceServerEvent, nil
}

func (p *Plugin) GetCommentDataWithAPIToken(webhookPayload *serializer.ConfluenceServerWebhookPayload, pluginConfig *config.Configuration) (*CommentResponse, error) {
	commentResponse := &CommentResponse{}
	path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?expand=body.view,container,space,history", PathContentData, strconv.FormatInt(webhookPayload.Comment.ID, 10)))

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, err
	}

	if err := json.Unmarshal(body, commentResponse); err != nil {
		return nil, errors.Wrapf(err, "error getting comment data with API token")
	}

	commentResponse.Body.View.Value = util.GetBodyForExcerpt(commentResponse.Body.View.Value)

	return commentResponse, nil
}

func (p *Plugin) GetPageDataWithAPIToken(pageID int, pluginConfig *config.Configuration) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?status=any&expand=body.view,container,space,history", PathContentData, strconv.Itoa(pageID)))

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, err
	}

	if err := json.Unmarshal(body, pageResponse); err != nil {
		return nil, errors.Wrapf(err, "error getting page data with API token")
	}

	pageResponse.Body.View.Value = util.GetBodyForExcerpt(pageResponse.Body.View.Value)

	return pageResponse, nil
}

func (p *Plugin) GetSpaceDataWithAPIToken(spaceKey string, pluginConfig *config.Configuration) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?status=any", PathSpaceData, spaceKey))

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, err
	}

	if err := json.Unmarshal(body, spaceResponse); err != nil {
		return nil, errors.Wrapf(err, "error getting space data with APIToken")
	}

	return spaceResponse, nil
}

func (p *Plugin) MakeHTTPCallWithAPIToken(path string) ([]byte, int, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	err = p.SetAdminAPITokenRequestHeader(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return body, resp.StatusCode, err
}

func (p *Plugin) SetAdminAPITokenRequestHeader(req *http.Request) error {
	pluginConfig := config.GetConfig()

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pluginConfig.AdminAPIToken))
	req.Header.Set("Accept", "application/json")

	return nil
}

func respondToTestConnection(body []byte) bool {
	var testConnectionBody struct {
		Test bool `json:"test"`
	}

	if err := json.Unmarshal(body, &testConnectionBody); err != nil {
		return false
	}

	return testConnectionBody.Test
}
