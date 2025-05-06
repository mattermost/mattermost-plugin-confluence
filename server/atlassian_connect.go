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
	Path:    "/atlassian-connect.json",
	Method:  http.MethodGet,
	Execute: renderAtlassianConnectJSON,
}

func renderAtlassianConnectJSON(w http.ResponseWriter, r *http.Request, p *Plugin) {
	conf := config.GetConfig()
	if status, err := verifyHTTPSecret(conf.Secret, r.FormValue("secret")); err != nil {
		config.Mattermost.LogWarn("Failed to verify secret", "error", err.Error())
		http.Error(w, "invalid secret", status)
		return
	}

	bundlePath, err := config.Mattermost.GetBundlePath()
	if err != nil {
		config.Mattermost.LogWarn("Failed to get bundle path.", "Error", err.Error())
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
		p.client.Log.Error("Error parsing the template", "error", err.Error())
		http.Error(w, "error rendering the template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = tmpl.Execute(w, values); err != nil {
		p.client.Log.Error("Error writng the template as response", "error", err.Error())
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}
}
