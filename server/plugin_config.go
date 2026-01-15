package main

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

var getPluginConfig = &Endpoint{
	Path:            "/config",
	Method:          http.MethodGet,
	Execute:         handleGetPluginConfig,
	IsAuthenticated: true,
}

type PluginConfigResponse struct {
	SupportedEvents []EventConfig `json:"supportedEvents"`
}

type EventConfig struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func handleGetPluginConfig(w http.ResponseWriter, r *http.Request, p *Plugin) {
	pluginConfig := config.GetConfig()

	// Get supported events based on version
	supportedEventStrings := serializer.GetSupportedEvents(pluginConfig.ServerVersionGreaterthan9)

	// Convert to EventConfig format for frontend
	supportedEvents := make([]EventConfig, 0, len(supportedEventStrings))
	for _, event := range supportedEventStrings {
		supportedEvents = append(supportedEvents, EventConfig{
			Value: event,
			Label: serializer.EventDisplayName(event),
		})
	}

	response := PluginConfigResponse{
		SupportedEvents: supportedEvents,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.client.Log.Error("Error encoding plugin config response", "error", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
