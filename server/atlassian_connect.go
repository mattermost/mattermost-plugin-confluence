package main

import (
	"html/template"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

var atlassianConnectJSON = &Endpoint{
	Path:            "/atlassian-connect.json",
	Method:          http.MethodGet,
	Execute:         renderAtlassianConnectJSON,
	IsAuthenticated: false,
}

func renderAtlassianConnectJSON(w http.ResponseWriter, r *http.Request, p *Plugin) {
	conf := config.GetConfig()

	if status, err := verifyHTTPSecret(conf.Secret, r.FormValue("secret")); err != nil {
		p.client.Log.Error("Failed to verify secret for Atlassian Connect JSON", "error", err.Error())
		http.Error(w, "Invalid secret", status)
		return
	}

	bundlePath, err := config.Mattermost.GetBundlePath()
	if err != nil {
		p.client.Log.Error("Failed to get bundle path", "error", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	templateDir := filepath.Join(bundlePath, "assets", "templates")
	tmplPath := path.Join(templateDir, "atlassian-connect.json")
	values := map[string]string{
		"BaseURL":      util.GetPluginURL(),
		"RouteACJSON":  util.GetAtlassianConnectURLPath(),
		"ExternalURL":  util.GetSiteURL(),
		"PluginKey":    util.GetPluginKey(),
		"SharedSecret": url.QueryEscape(conf.Secret),
	}

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		p.client.Log.Error("Error parsing Atlassian Connect JSON template", "error", err.Error())
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = tmpl.Execute(w, values); err != nil {
		p.client.Log.Error("Error writing Atlassian Connect JSON response", "error", err.Error())
		http.Error(w, "Failed to generate response", http.StatusInternalServerError)
		return
	}
}
