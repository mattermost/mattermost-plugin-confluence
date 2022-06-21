package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func (p *Plugin) handleGetSpacesForConfluenceURL(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(config.HeaderMattermostUserID)

	client, err := p.GetClientFromURL(r.URL.Path, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	spaces, err := client.GetSpaces()
	if err != nil {
		p.LogAndRespondError(w, http.StatusInternalServerError, "not able to get Spaces for confluence url.", err)
		return
	}
	responseBody, err := json.Marshal(spaces)
	if err != nil {
		p.LogAndRespondError(w, http.StatusInternalServerError, "not able to get marshal spaces.", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(string(responseBody)))
}
