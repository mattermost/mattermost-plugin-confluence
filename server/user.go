package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	AdminMattermostUserID = "admin"
)

func httpOAuth2Connect(w http.ResponseWriter, r *http.Request, p *Plugin) {
	if r.Method != http.MethodGet {
		_, _ = respondErr(w, http.StatusMethodNotAllowed,
			errors.New("method "+r.Method+" is not allowed, must be GET"))
		return
	}

	isAdmin := IsAdmin(w, r)

	mattermostUserID := r.Header.Get("Mattermost-User-Id")
	if mattermostUserID == "" {
		_, _ = respondErr(w, http.StatusUnauthorized,
			errors.New("not authorized"))
		return
	}

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	if instanceURL == "" {
		http.Error(w, "missing Confluence base url. Please run `/confluence install server`", http.StatusInternalServerError)
		return
	}

	connection, err := store.LoadConnection(instanceURL, mattermostUserID)
	if err == nil && len(connection.ConfluenceAccountID()) != 0 {
		_, _ = respondErr(w, http.StatusBadRequest,
			errors.New("you already have a Confluence account linked to your Mattermost account. Please use `/confluence disconnect` to disconnect"))
		return
	} else if err != nil {
		p.client.Log.Error("Error loading the connection", "UserID", mattermostUserID, "InstanceURL", instanceURL, "error", err.Error())
		_, _ = respondErr(w, http.StatusInternalServerError, errors.New("error occurred while connecting user to Confluence"))
		return
	}

	redirectURL, err := p.getUserConnectURL(instanceURL, mattermostUserID, isAdmin)
	if err != nil {
		_, _ = respondErr(w, http.StatusInternalServerError, errors.New("error occurred while connecting user to Confluence"))
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func httpOAuth2Complete(w http.ResponseWriter, r *http.Request, p *Plugin) {
	var err error
	var status int
	// Prettify error output
	defer func() {
		if err == nil {
			return
		}

		errText := err.Error()
		if len(errText) > 0 {
			errText = strings.ToUpper(errText[:1]) + errText[1:]
		}
		status, err = p.respondTemplate(w, "/other/message.html", nil, status, "text/html", struct {
			Header  string
			Message string
		}{
			Header:  "Failed to connect to Confluence.",
			Message: errText,
		})
	}()

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "missing authorization state", http.StatusBadRequest)
		return
	}

	mattermostUserID := r.Header.Get(config.HeaderMattermostUserID)
	if mattermostUserID == "" {
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	if instanceURL == "" {
		http.Error(w, "missing Confluence base url", http.StatusInternalServerError)
		return
	}

	isAdmin := IsAdmin(w, r)

	cuser, mmuser, err := p.CompleteOAuth2(mattermostUserID, code, state, instanceURL, isAdmin)
	if err != nil {
		http.Error(w, "Failed to complete OAuth2 connection", http.StatusInternalServerError)
		return
	}

	_, _ = p.respondTemplate(w, "", r, http.StatusOK, "text/html", struct {
		MattermostDisplayName string
		ConfluenceDisplayName string
	}{
		ConfluenceDisplayName: cuser.DisplayName + " (" + cuser.Name + ")",
		MattermostDisplayName: mmuser.GetDisplayName(model.ShowNicknameFullName),
	})
}

func (p *Plugin) CompleteOAuth2(mattermostUserID, code, state string, instanceID string, isAdmin bool) (*types.ConfluenceUser, *model.User, error) {
	if mattermostUserID == "" || code == "" || state == "" {
		return nil, nil, errors.New("missing user, code or state")
	}

	if err := store.VerifyOAuth2State(state); err != nil {
		p.client.Log.Error("Error verifying OAuth2 state", "State", state, "error", err.Error())
		return nil, nil, errors.WithMessage(err, "missing stored state")
	}

	mmuser, appErr := p.API.GetUser(mattermostUserID)
	if appErr != nil {
		return nil, nil, fmt.Errorf("failed to load user %s", mattermostUserID)
	}

	oconf, err := p.GetServerOAuth2Config(instanceID, isAdmin)
	if err != nil {
		p.client.Log.Error("Error getting server OAuth2 config", "InstanceID", instanceID, "error", err.Error())
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tok, err := oconf.Exchange(ctx, code)
	if err != nil {
		p.client.Log.Error("Error converting authorization code into token", "error", err.Error())
		return nil, nil, err
	}

	encryptedToken, err := p.NewEncodedAuthToken(tok)
	if err != nil {
		return nil, nil, err
	}

	connection := &types.Connection{
		OAuth2Token:      encryptedToken,
		IsAdmin:          isAdmin,
		MattermostUserID: mattermostUserID,
	}

	client, err := p.GetServerClient(instanceID, connection)
	if err != nil {
		p.client.Log.Error("Error getting server client", "InstanceID", instanceID, "error", err.Error())
		return nil, nil, err
	}

	confluenceUser, err := client.GetSelf()
	if err != nil {
		p.client.Log.Error("Error getting the Confluence user from client", "error", err.Error())
		return nil, nil, err
	}
	connection.ConfluenceUser = *confluenceUser

	err = p.connectUser(instanceID, mattermostUserID, connection)
	if err != nil {
		return nil, nil, err
	}

	return &connection.ConfluenceUser, mmuser, nil
}

func (p *Plugin) getUserConnectURL(instanceID string, mattermostUserID string, isAdmin bool) (string, error) {
	conf, err := p.GetServerOAuth2Config(instanceID, isAdmin)
	if err != nil {
		p.client.Log.Error("Error getting server OAuth2 config", "InstanceID", instanceID, "error", err.Error())
		return "", err
	}
	state := fmt.Sprintf("%v_%v", model.NewId()[0:15], mattermostUserID)
	if isAdmin {
		state = fmt.Sprintf("%v_%v", state, AdminMattermostUserID)
	}
	if err = store.StoreOAuth2State(state); err != nil {
		p.client.Log.Error("Error storing the OAuth2 state", "InstanceID", instanceID, "State", state, "error", err.Error())
		return "", err
	}

	return conf.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (p *Plugin) DisconnectUser(instanceURL string, mattermostUserID string) (*types.Connection, error) {
	user, err := store.LoadUser(mattermostUserID)
	if err != nil {
		p.client.Log.Error("Error loading the user", "UserID", user.MattermostUserID, "error", err.Error())
		return nil, err
	}

	return p.disconnectUser(instanceURL, user)
}

func (p *Plugin) disconnectUser(instanceID string, user *types.User) (*types.Connection, error) {
	if user.InstanceURL != instanceID {
		return nil, errors.Wrapf(store.ErrNotFound, "user is not connected to %q", instanceID)
	}

	conn, err := store.LoadConnection(instanceID, user.MattermostUserID)
	if err != nil {
		p.client.Log.Error("Error loading the connection", "UserID", user.MattermostUserID, "InstanceURL", instanceID, "error", err.Error())
		return nil, err
	}

	if user.InstanceURL == instanceID {
		user.InstanceURL = ""
	}

	if err = store.DeleteConnection(instanceID, user.MattermostUserID); err != nil && errors.Cause(err) != store.ErrNotFound {
		p.client.Log.Error("Error deleting the connection", "UserID", user.MattermostUserID, "error", err.Error())
		return nil, err
	}

	if err = store.StoreUser(user); err != nil {
		p.client.Log.Error("Error storing the user", "UserID", user.MattermostUserID, "error", err.Error())
		return nil, err
	}

	return conn, nil
}

func (p *Plugin) connectUser(instanceID, mattermostUserID string, connection *types.Connection) error {
	user, err := store.LoadUser(mattermostUserID)
	if err != nil {
		if errors.Cause(err) != store.ErrNotFound {
			p.client.Log.Error("Error storing the user", "UserID", user.MattermostUserID, "error", err.Error())
			return err
		}
		user = types.NewUser(mattermostUserID)
	}
	user.InstanceURL = instanceID

	if err = store.StoreConnection(instanceID, mattermostUserID, connection); err != nil {
		p.client.Log.Error("Error storing connection", "InstanceID", instanceID, "UserID", mattermostUserID, "error", err.Error())
		return err
	}

	if err = store.StoreConnection(instanceID, mattermostUserID, connection); err != nil {
		p.client.Log.Error("Error storing connection", "InstanceID", instanceID, "UserID", mattermostUserID, "error", err.Error())
		return err
	}

	if err = store.StoreConnection(instanceID, AdminMattermostUserID, connection); err != nil {
		p.client.Log.Error("Error storing connection", "InstanceID", instanceID, "UserID", mattermostUserID, "error", err.Error())
		return err
	}

	if err = store.StoreUser(user); err != nil {
		p.client.Log.Error("Error storing the user", "UserID", user.MattermostUserID, "error", err.Error())
		return err
	}

	if err = p.flowManager.StartCompletionWizard(mattermostUserID); err != nil {
		return err
	}

	return nil
}

// refreshAndStoreToken checks whether the current access token is expired or not. If it is,
// then it refreshes the token and stores the new pair of access and refresh tokens in kv store.
func (p *Plugin) refreshAndStoreToken(connection *types.Connection, instanceID string, oconf *oauth2.Config) (*oauth2.Token, error) {
	token, err := p.ParseAuthToken(connection.OAuth2Token)
	if err != nil {
		return nil, err
	}

	// If there is only one minute left for the token to expire, we are refreshing the token.
	// We don't want the token to expire between the time when we decide that the old token is valid
	// and the time at which we create the request. We are handling that by not letting the token expire.
	if time.Until(token.Expiry) > 1*time.Minute {
		return token, nil
	}

	src := oconf.TokenSource(context.Background(), token)
	newToken, err := src.Token() // this actually goes and renews the tokens
	if err != nil {
		return nil, errors.Wrap(err, "unable to get the new refreshed token")
	}
	if newToken.AccessToken != token.AccessToken {
		encryptedToken, err := p.NewEncodedAuthToken(newToken)
		if err != nil {
			return nil, err
		}
		connection.OAuth2Token = encryptedToken

		if err = store.StoreConnection(instanceID, connection.MattermostUserID, connection); err != nil {
			p.client.Log.Error("Error storing the connection", "InstanceID", instanceID, "UserID", connection.MattermostUserID, "error", err.Error())
			return nil, err
		}

		if connection.IsAdmin {
			if err = store.StoreConnection(instanceID, AdminMattermostUserID, connection); err != nil {
				p.client.Log.Error("Error storing the connection", "InstanceID", instanceID, "UserID", connection.MattermostUserID, "error", err.Error())
				return nil, err
			}
		}
		return newToken, nil
	}

	return token, nil
}

type UserConnectionInfo struct {
	CanRunSubscribeCommand    bool `json:"can_run_subscribe_command"`
	ServerVersionGreaterthan9 bool `json:"server_version_greater_than_9"`
}

func httpGetUserInfo(w http.ResponseWriter, r *http.Request, p *Plugin) {
	if r.Method != http.MethodGet {
		_, _ = respondErr(w, http.StatusMethodNotAllowed,
			errors.New("method "+r.Method+" is not allowed, must be GET"))
		return
	}

	mattermostUserID := r.Header.Get("Mattermost-User-Id")
	if mattermostUserID == "" {
		_, _ = respondErr(w, http.StatusUnauthorized,
			errors.New("not authorized"))
		return
	}

	serverVersionGreaterThan9 := config.GetConfig().ServerVersionGreaterthan9

	if !serverVersionGreaterThan9 {
		info := &UserConnectionInfo{
			CanRunSubscribeCommand:    util.IsSystemAdmin(mattermostUserID),
			ServerVersionGreaterthan9: serverVersionGreaterThan9,
		}
		b, _ := json.Marshal(info)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
		return
	}

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	if instanceURL == "" {
		http.Error(w, "missing Confluence base url. Please run `/confluence install server`", http.StatusInternalServerError)
		return
	}

	connection, err := store.LoadConnection(instanceURL, mattermostUserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			info := &UserConnectionInfo{
				CanRunSubscribeCommand:    false,
				ServerVersionGreaterthan9: serverVersionGreaterThan9,
			}
			b, _ := json.Marshal(info)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
			return
		}

		p.client.Log.Error("Error getting client connection", "MattermostUserID", mattermostUserID, "error", err)
		http.Error(w, "Error occurred while checking user connection status", http.StatusInternalServerError)
		return
	}

	info := &UserConnectionInfo{
		CanRunSubscribeCommand:    len(connection.ConfluenceAccountID()) != 0,
		ServerVersionGreaterthan9: serverVersionGreaterThan9,
	}

	b, _ := json.Marshal(info)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(string(b)))
}
