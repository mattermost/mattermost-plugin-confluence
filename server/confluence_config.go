package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const (
	ParamUserID     = "userID"
	ErrorInvalidURL = "Please enter a valid URL."
)

func (p *Plugin) handleConfluenceConfig(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	channelID := pathParams[ParamChannelID]
	userID := pathParams[ParamUserID]
	if !model.IsValidId(channelID) || !model.IsValidId(userID) {
		http.Error(w, "Invalid channel or user id", http.StatusBadRequest)
		p.API.LogError("Invalid channel or user id")
		return
	}

	decoder := json.NewDecoder(r.Body)
	submitRequest := &model.SubmitDialogRequest{}
	response := &model.SubmitDialogResponse{}
	if err := decoder.Decode(&submitRequest); err != nil {
		p.API.LogError("Error decoding the submit dialog request.", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	configInstanceURL := submitRequest.Submission[configServerURL].(string)
	if vErr := utils.IsValidURL(configInstanceURL); vErr != nil {
		response.Error = ErrorInvalidURL
		response, mErr := json.Marshal(response)
		if mErr != nil {
			p.API.LogError("Error in marshaling the response.", "Error", mErr.Error())
			http.Error(w, mErr.Error(), http.StatusInternalServerError)
			return
		}

		p.API.LogError("Error in validating the URL.", "Error", vErr.Error())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}

	config := &serializer.ConfluenceConfig{
		ServerURL:    strings.TrimSuffix(configInstanceURL, "/"),
		ClientID:     submitRequest.Submission[configClientID].(string),
		ClientSecret: submitRequest.Submission[configClientSecret].(string),
	}

	if err := p.instanceStore.StoreInstanceConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		p.API.LogError("Error storing instance config.", "Error", err.Error())
		return
	}

	post := &model.Post{
		UserId:    p.conf.botUserID,
		ChannelId: channelID,
		Message:   fmt.Sprintf("Your config is saved for confluence instance %s", config.ServerURL),
	}

	_ = p.API.SendEphemeralPost(userID, post)
	ReturnStatusOK(w)
}
