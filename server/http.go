package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"

	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const (
	routeUserConnect  = "/oauth2/connect"
	routeUserComplete = "/oauth2/complete.html"
)

const (
	routePrefixInstance = "instance"
)

const (
	websocketEventInstanceStatus = "instance_status"
	websocketEventConnect        = "connect"
	websocketEventDisconnect     = "disconnect"
)

// Endpoints is a map of endpoint key to endpoint object
// Usage: getEndpointKey(GetMetadata): GetMetadata
var Endpoints = map[string]*Endpoint{
	getEndpointKey(GetChannelSubscription): GetChannelSubscription,
}

// Uniquely identifies an endpoint using path and method
func getEndpointKey(endpoint *Endpoint) string {
	return utils.GetKeyHash(endpoint.Path + "-" + endpoint.Method)
}

// InitAPI initializes the REST API
func (p *Plugin) InitAPI() *mux.Router {
	r := mux.NewRouter()
	p.handleStaticFiles(r)

	s := r.PathPrefix("/api/v1").Subrouter()
	for _, endpoint := range Endpoints {
		handler := endpoint.Execute
		if endpoint.RequiresAdmin {
			handler = p.handleAdminRequired(endpoint)
		}
		s.HandleFunc(endpoint.Path, handler).Methods(endpoint.Method)
	}

	s.HandleFunc("/userinfo", p.httpGetUserInfo).Methods(http.MethodGet)
	s.HandleFunc("/atlassian-connect.json", p.renderAtlassianConnectJSON).Methods(http.MethodGet)
	s.HandleFunc("/cloud/{event:[A-Za-z0-9_]+}", p.handleConfluenceCloudWebhook).Methods(http.MethodPost)
	s.HandleFunc("/autocomplete/channel-subscriptions", p.handleGetChannelSubscriptions).Methods(http.MethodGet)
	s.HandleFunc("/autocomplete/configs", p.handleGetConfigList).Methods(http.MethodGet)
	s.HandleFunc("/autocomplete/installed-instances", p.handleGetInstanceList).Methods(http.MethodGet)
	s.HandleFunc("/config/{channelID:[A-Za-z0-9]+}/{userID:.+}", p.handleConfluenceConfig).Methods(http.MethodPost)

	apiRouter := s.PathPrefix("/instance/{instanceID:.+}").Subrouter()
	apiRouter.HandleFunc(routeUserConnect, p.httpOAuth2Connect).Methods(http.MethodGet)
	apiRouter.HandleFunc(routeUserComplete, p.httpOAuth2Complete).Methods(http.MethodGet)

	apiRouter.HandleFunc("/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}", p.handleSaveSubscription).Methods(http.MethodPost)
	apiRouter.HandleFunc("/{channelID:[A-Za-z0-9]+}/subscription/{type:[A-Za-z_]+}/{oldSubscriptionType:[A-Za-z_]+}", p.handleEditChannelSubscription).Methods(http.MethodPut)
	apiRouter.HandleFunc("/server/webhook/{userID:.+}", p.handleConfluenceServerWebhook).Methods(http.MethodPost)
	return r
}

// handleStaticFiles handles the static files under the assets directory.
func (p *Plugin) handleStaticFiles(r *mux.Router) {
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		p.API.LogWarn("Failed to get bundle path.", "Error", err.Error())
		return
	}

	// This will serve static files from the 'assets' directory under '/static/<filename>'
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(bundlePath, "assets")))))
}

func (p *Plugin) handleAdminRequired(endpoint *Endpoint) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if p.IsAdmin(w, r) {
			endpoint.Execute(w, r)
		}
	}
}

// IsAdmin verifies if provided request is performed by a logged-in Mattermost user.
func (p *Plugin) IsAdmin(w http.ResponseWriter, r *http.Request) bool {
	userID := r.Header.Get(config.HeaderMattermostUserID)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	return utils.IsSystemAdmin(userID)
}

func instancePath(route string, instanceID types.ID) string {
	encoded := url.PathEscape(encode([]byte(instanceID)))
	return path.Join("/"+routePrefixInstance+"/"+encoded, route)
}

func instancePathWithAdmin(route string, instanceID types.ID, isAdmin bool) string {
	encoded := url.PathEscape(encode([]byte(instanceID)))
	return path.Join("/"+routePrefixInstance+"/"+encoded, route) + "?admin=" + strconv.FormatBool(isAdmin)
}

func respondErr(w http.ResponseWriter, code int, err error) (int, error) {
	http.Error(w, err.Error(), code)
	return code, err
}

func (p *Plugin) loadTemplates(dir string) (map[string]*template.Template, error) {
	dir = filepath.Clean(dir)
	templates := make(map[string]*template.Template)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		template, err := template.ParseFiles(path)
		if err != nil {
			p.errorf("OnActivate: failed to parse template %s: %v", path, err)
			return nil
		}
		key := path[len(dir):]
		templates[key] = template
		p.debugf("loaded template %s", key)
		return nil
	})
	if err != nil {
		return nil, errors.WithMessage(err, "OnActivate: failed to load templates")
	}
	return templates, nil
}

func splitInstancePath(route string) (instanceURL string, remainingPath string) {
	leadingSlash := ""
	ss := strings.Split(route, "/")
	if len(ss) > 1 && ss[0] == "" {
		leadingSlash = "/"
		ss = ss[1:]
	}

	if len(ss) < 2 {
		return "", route
	}

	if ss[0] == "api" && strings.Contains(ss[1], "v") {
		ss = ss[2:]
	}

	if ss[0] != routePrefixInstance {
		return route, route
	}

	id, err := decode(ss[1])
	if err != nil {
		return "", route
	}
	return string(id), leadingSlash + strings.Join(ss[2:], "/")
}

func (p *Plugin) respondSpecialTemplate(w http.ResponseWriter, key string, status int, contentType string, values interface{}) (int, error) {
	w.Header().Set("Content-Type", contentType)
	t := p.templates[key]
	if t == nil {
		return respondErr(w, http.StatusInternalServerError,
			errors.New("no template found for "+key))
	}
	err := t.Execute(w, values)
	if err != nil {
		return http.StatusInternalServerError,
			errors.WithMessage(err, "failed to write response")
	}
	return status, nil
}

func (p *Plugin) respondTemplate(w http.ResponseWriter, r *http.Request, contentType string, values interface{}) (int, error) {
	_, path := splitInstancePath(r.URL.Path)
	w.Header().Set("Content-Type", contentType)
	t := p.templates[path]
	if t == nil {
		return respondErr(w, http.StatusInternalServerError,
			errors.New("no template found for "+path))
	}
	err := t.Execute(w, values)
	if err != nil {
		return http.StatusInternalServerError, errors.WithMessage(err, "failed to write response")
	}
	return http.StatusOK, nil
}

func respondJSON(w http.ResponseWriter, obj interface{}) (int, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, errors.WithMessage(err, "failed to marshal response"))
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		return http.StatusInternalServerError, errors.WithMessage(err, "failed to write response")
	}
	return http.StatusOK, nil
}
