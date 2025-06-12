package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	AdminMattermostUserID = "admin"
)

func httpOAuth2Connect(w http.ResponseWriter, r *http.Request, p *Plugin) {
	if r.Method != http.MethodGet {
		err := errors.New("method " + r.Method + " is not allowed, must be GET")
		p.client.Log.Error("Invalid HTTP method used. Method: %s. Error: %s", r.Method, err.Error())
		_, _ = respondErr(w, http.StatusMethodNotAllowed, err)
		return
	}

	isAdmin := IsAdmin(w, r)
	mattermostUserID := r.Header.Get(config.HeaderMattermostUserID)

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	if instanceURL == "" {
		p.client.Log.Error("Missing Confluence base URL. UserID: %s", mattermostUserID)
		http.Error(w, "Missing Confluence base URL. Please run `/confluence install server`.", http.StatusInternalServerError)
		return
	}

	connection, err := store.LoadConnection(instanceURL, mattermostUserID)
	if err == nil && len(connection.ConfluenceAccountID()) != 0 {
		p.client.Log.Info("User already has a Confluence account connected. UserID: %s", mattermostUserID)
		_, _ = respondErr(w, http.StatusBadRequest, errors.New("User already has a Confluence account linked. Use `/confluence disconnect` to unlink"))
		return
	}

	redirectURL, err := p.getUserConnectURL(instanceURL, mattermostUserID, isAdmin)
	if err != nil {
		p.client.Log.Error("Error generating user connect URL. UserID: %s. Error: %s", mattermostUserID, err.Error())
		_, _ = respondErr(w, http.StatusInternalServerError, errors.New("an error occurred while initiating Confluence connection. Please try again later"))
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func httpOAuth2Complete(w http.ResponseWriter, r *http.Request, p *Plugin) {
	var err error
	var status int

	// Prettify and present errors on the template page
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
		err = errors.New("missing authorization code")
		status = http.StatusBadRequest
		p.client.Log.Error("OAuth2 completion failed: %s", err.Error())
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		err = errors.New("missing authorization state")
		status = http.StatusBadRequest
		p.client.Log.Error("OAuth2 completion failed: %s", err.Error())
		return
	}

	instanceURL := config.GetConfig().GetConfluenceBaseURL()
	if instanceURL == "" {
		err = errors.New("missing Confluence base URL")
		status = http.StatusInternalServerError
		p.client.Log.Error("OAuth2 completion failed: %s", err.Error())
		return
	}

	isAdmin := IsAdmin(w, r)

	cuser, mmuser, completeErr := p.CompleteOAuth2(
		r.Header.Get(config.HeaderMattermostUserID),
		code,
		state,
		instanceURL,
		isAdmin,
	)
	if completeErr != nil {
		err = errors.New("an error occurred while completing the Confluence connection")
		status = http.StatusInternalServerError
		p.client.Log.Error("OAuth2 completion failed. Code: %s, State: %s. Error: %s", code, state, completeErr.Error())
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
		err := errors.New("method " + r.Method + " is not allowed, must be GET")
		p.client.Log.Error("Invalid HTTP method used in GetUserInfo. Error: %s", err.Error())
		_, _ = respondErr(w, http.StatusMethodNotAllowed, err)
		return
	}

	mattermostUserID := r.Header.Get(config.HeaderMattermostUserID)
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
		err := errors.New("missing Confluence base URL")
		p.client.Log.Error("Confluence base URL is not configured. Error: %s", err.Error())
		http.Error(w, "Confluence is not properly configured. Please contact the system administrator.", http.StatusInternalServerError)
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

		p.client.Log.Error("Failed to load user Confluence connection. MattermostUserID: %s. Error: %s", mattermostUserID, err.Error())
		http.Error(w, "Failed to retrieve user connection status. Please retry after some time.", http.StatusInternalServerError)
		return
	}

	info := &UserConnectionInfo{
		CanRunSubscribeCommand:    len(connection.ConfluenceAccountID()) != 0,
		ServerVersionGreaterthan9: serverVersionGreaterThan9,
	}

	b, _ := json.Marshal(info)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func (p *Plugin) hasChannelAccess(userID, channelID string) bool {
	_, err := p.API.GetChannelMember(channelID, userID)
	return err == nil
}

func (p *Plugin) validateUserConfluenceAccess(userID, confluenceURL, subscriptionType string, subscription serializer.Subscription) (int, error) {
	conn, err := store.LoadConnection(confluenceURL, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusUnauthorized, errors.New("User needs to connect their Confluence account")
		}
		p.client.Log.Error("Error loading connection for the user. ConfluenceURL: %s, UserID: %s. Error: %s", confluenceURL, userID, err.Error())
		return http.StatusInternalServerError, errors.New("unable to verify user's Confluence connection. Please try again later")
	}

	if conn.ConfluenceAccountID() == "" {
		return http.StatusUnauthorized, errors.New("User needs to connect their Confluence account")
	}

	client, err := p.GetServerClient(confluenceURL, conn)
	if err != nil {
		p.client.Log.Error("Error getting Confluence client. UserID: %s. Error: %s", userID, err.Error())
		return http.StatusInternalServerError, errors.New("An error occurred while connecting to Confluence. Please try again later")
	}

	serverClient, ok := client.(*confluenceServerClient)
	if !ok {
		p.client.Log.Error("Invalid Confluence server client type while validating user's Confluence access. UserID: %s", userID)
		return http.StatusInternalServerError, errors.New("an unexpected error occurred. Please try again later")
	}

	switch subscriptionType {
	case serializer.SubscriptionTypeSpace:
		spaceSub, ok := subscription.(serializer.SpaceSubscription)
		if !ok {
			p.client.Log.Error("Failed to parse space subscription. UserID: %s", userID)
			return http.StatusBadRequest, errors.New("invalid space subscription details provided")
		}
		if _, err = serverClient.GetSpaceData(spaceSub.SpaceKey); err != nil {
			p.client.Log.Error("User does not have access to the space. UserID: %s, SpaceKey: %s. Error: %s", userID, spaceSub.SpaceKey, err.Error())
			return http.StatusForbidden, errors.New("User does not have an access to this Confluence space")
		}

	case serializer.SubscriptionTypePage:
		pageSub, ok := subscription.(serializer.PageSubscription)
		if !ok {
			p.client.Log.Error("Failed to parse page subscription. UserID: %s", userID)
			return http.StatusBadRequest, errors.New("invalid page subscription details provided")
		}
		pageID, err := strconv.Atoi(pageSub.PageID)
		if err != nil {
			p.client.Log.Error("Error converting PageID to integer. UserID: %s, PageID: %s. Error: %s", userID, pageSub.PageID, err.Error())
			return http.StatusInternalServerError, errors.New("an error occurred while processing the page details. Please try again later")
		}

		if _, err := serverClient.GetPageData(pageID); err != nil {
			p.client.Log.Error("User does not have access to the page. UserID: %s, PageID: %d. Error: %s", userID, pageID, err.Error())
			return http.StatusForbidden, errors.New("User does not have an access to this Confluence page")
		}

	default:
		p.client.Log.Error("Unknown subscription type. UserID: %s, Type: %s", userID, subscriptionType)
		return http.StatusBadRequest, errors.New("unsupported subscription type")
	}

	return http.StatusOK, nil
}
