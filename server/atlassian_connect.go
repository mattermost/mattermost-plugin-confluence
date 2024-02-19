package main

import (
	"html/template"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func (p *Plugin) renderAtlassianConnectJSON(w http.ResponseWriter, r *http.Request) {
	if status, err := verifyHTTPSecret(p.conf.Secret, r.FormValue("secret")); err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		config.Mattermost.LogWarn("Failed to get bundle path.", "Error", err.Error())
		return
	}
	templateDir := filepath.Join(bundlePath, "assets", "templates")
	tmplPath := path.Join(templateDir, "atlassian-connect.json")
	values := map[string]string{
		"BaseURL":      p.GetPluginURL(),
		"RouteACJSON":  p.GetAtlassianConnectURLPath(),
		"ExternalURL":  p.GetSiteURL(),
		"PluginKey":    p.GetPluginKey(),
		"SharedSecret": url.QueryEscape(p.conf.Secret),
	}
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = tmpl.Execute(w, values); err != nil {
		http.Error(w, "failed to write response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
