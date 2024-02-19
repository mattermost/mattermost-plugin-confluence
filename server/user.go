package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/kvstore"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type User struct {
	PluginVersion      string
	MattermostUserID   types.ID   `json:"mattermost_user_id"`
	ConnectedInstances *Instances `json:"connected_instances,omitempty"`
	DefaultInstanceID  types.ID   `json:"default_instance_id,omitempty"`
}

type ConfluenceUser struct {
	AccountID   string `json:"accountId,omitempty"`
	Name        string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type UserGroups struct {
	Groups []*UserGroup `json:"results,omitempty"`
}

type UserGroup struct {
	Name string `json:"name"`
}

type Connection struct {
	ConfluenceUser
	PluginVersion     string
	OAuth2Token       string `json:"token,omitempty"`
	DefaultProjectKey string `json:"default_project_key,omitempty"`
	IsAdmin           bool   `json:"is_admin,omitempty"`
	MattermostUserID  string `json:"mattermost_user_id,omitempty"`
}

func (c *Connection) ConfluenceAccountID() types.ID {
	if c.AccountID != "" {
		return types.ID(c.AccountID)
	}

	return types.ID(c.Name)
}

func NewUser(mattermostUserID types.ID) *User {
	return &User{
		MattermostUserID:   mattermostUserID,
		ConnectedInstances: NewInstances(),
	}
}

func (p *Plugin) httpOAuth2Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		_, _ = respondErr(w, http.StatusMethodNotAllowed,
			errors.New("method "+r.Method+" is not allowed, must be GET"))
		return
	}

	isAdminParam := r.URL.Query().Get(AdminMattermostUserID)
	if isAdminParam == "" {
		http.Error(w, "missing isAdmin query param", http.StatusBadRequest)
		return
	}

	isAdmin, err := strconv.ParseBool(isAdminParam)
	if err != nil {
		_, _ = respondErr(w, http.StatusInternalServerError, err)
		return
	}

	mattermostUserID := r.Header.Get("Mattermost-User-Id")
	if mattermostUserID == "" {
		_, _ = respondErr(w, http.StatusUnauthorized,
			errors.New("not authorized"))
		return
	}

	instance, err := p.getInstanceFromURL(r.URL.Path)
	if err != nil {
		_, _ = respondErr(w, http.StatusInternalServerError, err)
		return
	}

	connection, err := p.userStore.LoadConnection(instance.GetID(), types.ID(mattermostUserID))
	if err == nil && len(connection.ConfluenceAccountID()) != 0 {
		_, _ = respondErr(w, http.StatusBadRequest,
			errors.New("you already have a Confluence account linked to your Mattermost account. Please use `/confluence disconnect` to disconnect"))
		return
	}

	redirectURL, err := p.getUserConnectURL(instance, mattermostUserID, isAdmin)
	if err != nil {
		_, _ = respondErr(w, http.StatusInternalServerError, err)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (p *Plugin) getUserConnectURL(instance Instance, mattermostUserID string, isAdmin bool) (string, error) {
	conf, err := instance.GetOAuth2Config(isAdmin)
	if err != nil {
		return "", err
	}
	state := fmt.Sprintf("%v_%v", model.NewId()[0:15], mattermostUserID)
	if isAdmin {
		state = fmt.Sprintf("%v_%v", state, AdminMattermostUserID)
	}
	err = p.otsStore.StoreOAuth2State(state)
	if err != nil {
		return "", err
	}

	return conf.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (p *Plugin) httpOAuth2Complete(w http.ResponseWriter, r *http.Request) {
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
		status, err = p.respondSpecialTemplate(w, "/other/message.html", status, "text/html", struct {
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
		http.Error(w, "not authorized", http.StatusInternalServerError)
		return
	}

	instance, err := p.getInstanceFromURL(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isAdmin := false
	if strings.Contains(state, AdminMattermostUserID) {
		isAdmin = true
	}

	cuser, mmuser, err := p.CompleteOAuth2(mattermostUserID, code, state, instance, isAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = p.respondTemplate(w, r, "text/html", struct {
		MattermostDisplayName string
		ConfluenceDisplayName string
	}{
		ConfluenceDisplayName: cuser.DisplayName + " (" + cuser.Name + ")",
		MattermostDisplayName: mmuser.GetDisplayName(model.ShowNicknameFullName),
	})
}

func (p *Plugin) CompleteOAuth2(mattermostUserID, code, state string, instance Instance, isAdmin bool) (*ConfluenceUser, *model.User, error) {
	if mattermostUserID == "" || code == "" || state == "" {
		return nil, nil, errors.New("missing user, code or state")
	}

	err := p.otsStore.VerifyOAuth2State(state)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "missing stored state")
	}

	mmuser, appErr := p.API.GetUser(mattermostUserID)
	if appErr != nil {
		return nil, nil, fmt.Errorf("failed to load user %s", mattermostUserID)
	}

	oconf, err := instance.GetOAuth2Config(isAdmin)
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()
	tok, err := oconf.Exchange(ctx, code)
	if err != nil {
		return nil, nil, err
	}

	encryptedToken, err := p.NewEncodedAuthToken(tok)
	if err != nil {
		return nil, nil, err
	}

	connection := &Connection{
		PluginVersion:    manifest.Version,
		OAuth2Token:      encryptedToken,
		IsAdmin:          isAdmin,
		MattermostUserID: mattermostUserID,
	}

	client, err := instance.GetClient(connection)
	if err != nil {
		return nil, nil, err
	}

	// Fetch the cloudid for cloud instance if not already present
	if instance.Common().Type == CloudInstanceType && instance.(*cloudInstance).CloudID == "" {
		cloudID, cErr := client.(*confluenceCloudClient).GetCloudID()
		if cErr != nil {
			return nil, nil, cErr
		}
		ci := instance.(*cloudInstance)
		ci.CloudID = cloudID
		// Update the instance stored in the store
		if err = p.InstallInstance(ci, true); err != nil {
			return nil, nil, err
		}
		// Create a client with new base URL containing cloudID
		client, err = instance.GetClient(connection)
		if err != nil {
			return nil, nil, err
		}
	}

	confluenceUser, err := client.GetSelf()
	if err != nil {
		return nil, nil, err
	}
	connection.ConfluenceUser = *confluenceUser

	err = p.connectUser(instance, types.ID(mattermostUserID), connection)
	if err != nil {
		return nil, nil, err
	}

	p.track("userConnected", mattermostUserID)

	return &connection.ConfluenceUser, mmuser, nil
}

func (p *Plugin) getInstanceFromURL(instanceURL string) (Instance, error) {
	instanceURL, _ = splitInstancePath(instanceURL)

	instanceID, err := p.ResolveWebhookInstanceURL(instanceURL)
	if err != nil {
		return nil, err
	}
	instance, err := p.instanceStore.LoadInstance(instanceID)
	if err != nil {
		return nil, fmt.Errorf("error occurred while loading instance from store. InstanceID: %s", instanceID)
	}
	return instance, nil
}

func (p *Plugin) DisconnectUser(instanceURL string, mattermostUserID types.ID) (*Connection, error) {
	user, instance, err := p.LoadUserInstance(mattermostUserID, instanceURL)
	if err != nil {
		return nil, err
	}
	return p.disconnectUser(instance, user)
}

func (p *Plugin) disconnectUser(instance Instance, user *User) (*Connection, error) {
	if !user.ConnectedInstances.Contains(instance.GetID()) {
		return nil, errors.Wrapf(kvstore.ErrNotFound, "user is not connected to %q", instance.GetID())
	}
	conn, err := p.userStore.LoadConnection(instance.GetID(), user.MattermostUserID)
	if err != nil {
		return nil, err
	}

	if user.DefaultInstanceID == instance.GetID() {
		user.DefaultInstanceID = ""
	}

	user.ConnectedInstances.Delete(instance.GetID())

	err = p.userStore.DeleteConnection(instance.GetID(), user.MattermostUserID)
	if err != nil && errors.Cause(err) != kvstore.ErrNotFound {
		return nil, err
	}

	err = p.userStore.StoreUser(user)
	if err != nil {
		return nil, err
	}

	info, err := p.GetUserInfo(user.MattermostUserID, user)
	if err != nil {
		return nil, err
	}
	p.API.PublishWebSocketEvent(websocketEventDisconnect, info.AsConfigMap(),
		&model.WebsocketBroadcast{UserId: user.MattermostUserID.String()})

	p.track("userDisconnected", user.MattermostUserID.String())

	return conn, nil
}

func (p *Plugin) connectUser(instance Instance, mattermostUserID types.ID, connection *Connection) error {
	user, err := p.userStore.LoadUser(mattermostUserID)
	if err != nil {
		if errors.Cause(err) != kvstore.ErrNotFound {
			return err
		}
		user = NewUser(mattermostUserID)
	}
	user.ConnectedInstances.Set(instance.Common())

	err = p.userStore.StoreConnection(instance.GetID(), mattermostUserID, connection)
	if err != nil {
		return err
	}
	client, err := instance.GetClient(connection)
	if err != nil {
		return err
	}

	if connection.IsAdmin {
		if _, err = client.(*confluenceServerClient).CheckConfluenceAdmin(); err != nil {
			return errors.New("user is not a confluence admin")
		}
		if err = p.userStore.StoreConnection(instance.GetID(), AdminMattermostUserID, connection); err != nil {
			return err
		}
	}
	err = p.userStore.StoreUser(user)
	if err != nil {
		return err
	}

	err = p.setupFlow.ForUser(string(mattermostUserID)).Start(nil)
	if err != nil {
		return errors.Wrap(err, "Failed to start wizard")
	}
	_ = p.setupFlow.ForUser(string(mattermostUserID)).Go(stepConnected)

	info, err := p.GetUserInfo(mattermostUserID, user)
	if err != nil {
		return err
	}

	p.API.PublishWebSocketEvent(websocketEventConnect, info.AsConfigMap(),
		&model.WebsocketBroadcast{UserId: mattermostUserID.String()},
	)

	p.track("userConnected", mattermostUserID.String())

	return nil
}

func (user *User) AsConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"mattermost_user_id":  user.MattermostUserID.String(),
		"connected_instances": user.ConnectedInstances.AsConfigMap(),
		"default_instance_id": user.DefaultInstanceID.String(),
	}
}

func inAllowedGroup(inGroups []*UserGroup, allowedGroups []string) bool {
	for _, inGroup := range inGroups {
		for _, allowedGroup := range allowedGroups {
			if strings.TrimSpace(inGroup.Name) == strings.TrimSpace(allowedGroup) {
				return true
			}
		}
	}
	return false
}

// HasPermissionToManageSubscription checks if MM user has permission to manage subscriptions in given channel.
// returns nil if the user has permission and a descriptive error otherwise.
func (p *Plugin) HasPermissionToManageSubscription(instanceID, userID, channelID string) error {
	if err := p.HasPermissionToManageSubscriptionForMattermostSide(userID, channelID); err != nil {
		return errors.New("do not have the permissions on mattermost side")
	}
	if err := p.HasPermissionToManageSubscriptionForConfluenceSide(instanceID, userID); err != nil {
		return errors.New("do not have the permissions on confluence side")
	}
	return nil
}

func (p *Plugin) HasPermissionToManageSubscriptionForMattermostSide(userID, channelID string) error {
	switch p.conf.RolesAllowedToEditConfluenceSubscriptions {
	case "team_admin":
		if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionManageTeam) {
			return errors.New("user is not team admin")
		}
	case "channel_admin":
		channel, appErr := p.API.GetChannel(channelID)
		if appErr != nil {
			return errors.Wrap(appErr, "unable to get channel to check permission")
		}
		switch channel.Type {
		case model.ChannelTypeOpen:
			if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionManagePublicChannelProperties) {
				return errors.New("user is not channel admin")
			}
		case model.ChannelTypePrivate:
			if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionManagePrivateChannelProperties) {
				return errors.New("user is not channel admin")
			}
		default:
			return errors.New("user can only subscribe in public and private channels")
		}
	case "users":
	default:
		if !p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
			return errors.New("user is not system admin")
		}
	}
	return nil
}

func (p *Plugin) HasPermissionToManageSubscriptionForConfluenceSide(instanceID, userID string) error {
	instance, err := p.getInstanceFromURL(instanceID)
	if err != nil {
		return errors.Wrap(err, "could not load confluence instance")
	}

	conn, err := p.userStore.LoadConnection(types.ID(instanceID), types.ID(userID))
	if err != nil {
		return errors.Wrap(err, "could not load confluence user")
	}

	if p.conf.GroupsAllowedToEditConfluenceSubscriptions == "" {
		return nil
	}

	client, err := instance.GetClient(conn)
	if err != nil {
		return errors.Wrap(err, "could not get an authenticated confluence client")
	}

	groups, err := client.GetUserGroups(conn)
	if err != nil {
		return errors.Wrap(err, "could not get confluence user groups")
	}

	allowedGroups := utils.Map(strings.Split(p.conf.GroupsAllowedToEditConfluenceSubscriptions, ","), strings.TrimSpace)
	if !inAllowedGroup(groups, allowedGroups) {
		return errors.New("user is not in allowed confluence user groups")
	}
	return nil
}

func (p *Plugin) CreateWebhook(instance Instance, subscription serializer.Subscription, userID string) error {
	adminConn, err := p.userStore.LoadConnection(types.ID(instance.GetURL()), types.ID(AdminMattermostUserID))
	if err != nil {
		return err
	}

	adminClient, err := instance.GetClient(adminConn)
	if err != nil {
		return err
	}

	redirectURL := fmt.Sprintf("%s/instance/%s/server/webhook/%s", p.GetPluginURL(), encode([]byte(instance.GetURL())), userID)

	totalSubscriptions, err := service.GetSubscriptionFromURL(instance.GetURL(), userID)
	if err != nil {
		return err
	}

	if totalSubscriptions == 0 {
		resp, err := adminClient.(*confluenceServerClient).CreateWebhook(subscription, redirectURL, p.conf.Secret)
		if err != nil {
			return err
		}
		err = p.userStore.StoreWebhookID(types.ID(instance.GetURL()), types.ID(userID), strconv.Itoa(resp.ID))
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) GetClientFromURL(url, userID string) (Client, error) {
	instance, err := p.getInstanceFromURL(url)
	if err != nil {
		return nil, err
	}

	conn, err := p.userStore.LoadConnection(types.ID(instance.GetURL()), types.ID(userID))
	if err != nil {
		return nil, err
	}

	client, err := instance.GetClient(conn)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// refreshAndStoreToken checks whether the current access token is expired or not. If it is,
// then it refreshes the token and stores the new pair of access and refresh tokens in kv store.
func (p *Plugin) refreshAndStoreToken(connection *Connection, instanceID types.ID, oconf *oauth2.Config) (*oauth2.Token, error) {
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

		err = p.userStore.StoreConnection(instanceID, types.ID(connection.MattermostUserID), connection)
		if err != nil {
			return nil, err
		}

		if connection.IsAdmin {
			if err = p.userStore.StoreConnection(instanceID, AdminMattermostUserID, connection); err != nil {
				return nil, err
			}
		}
		return newToken, nil
	}

	return token, nil
}
