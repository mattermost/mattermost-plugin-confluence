package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const paramUserID = "user_id"

func (p *Plugin) handleGetConfigList(w http.ResponseWriter, r *http.Request) {
	userID := r.FormValue(paramUserID)
	if !utils.IsSystemAdmin(userID) {
		http.Error(w, "user is not a system admin", http.StatusUnauthorized)
		return
	}

	configKeys, err := p.GetConfigKeyList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := []model.AutocompleteListItem{}
	for _, key := range configKeys {
		key = strings.Split(key, "_")[0]
		key = strings.TrimSuffix(key, "/")
		out = append(out, model.AutocompleteListItem{
			Item: key,
		})
	}

	response, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(response)
}
