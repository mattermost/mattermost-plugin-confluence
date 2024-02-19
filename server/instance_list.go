package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) handleGetInstanceList(w http.ResponseWriter, r *http.Request) {
	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := []model.AutocompleteListItem{}
	for _, ID := range instances.IDs() {
		out = append(out, model.AutocompleteListItem{
			Item: ID.String(),
		})
	}

	response, _ := json.Marshal(out)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(response)
}
