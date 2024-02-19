package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	httpOAuth2ConnectURL  = "/api/v1%s"
	httpOAuth2CompleteURL = "/api/v1/instance/%s/oauth2/complete.html?code=testcode&state=testState"
	correctInstanceID     = "Y29uZmx1ZW5jZXVybDE="
	wrongInstanceID       = "wrongInstanceID"
)

func TestHttpOAuth2Connect(t *testing.T) {
	tests := map[string]struct {
		method     string
		userID     string
		statusCode int
		isAdmin    bool
	}{
		"wrong API method": {
			method:     "POST",
			userID:     "non_connected_user",
			statusCode: http.StatusMethodNotAllowed,
			isAdmin:    false},
		"user already connected": {
			method:     "GET",
			userID:     "connected_user",
			statusCode: http.StatusFound,
			isAdmin:    false},
		"user not connected to confluence will atempt connect without admin access": {
			method:     "GET",
			userID:     "non_connected_user",
			statusCode: http.StatusFound,
			isAdmin:    false},
		"user not connected to confluence will atempt connect with admin access": {
			method:     "GET",
			userID:     "non_connected_user",
			statusCode: http.StatusFound,
			isAdmin:    true},
	}
	mockAPI := baseMock()
	mockAPI.On("LogError", mockAnythingOfTypeBatch("string", 13)...).Return(nil)

	mockAPI.On("LogDebug", mockAnythingOfTypeBatch("string", 11)...).Return(nil)

	p := Plugin{}
	p.SetAPI(mockAPI)

	mockAPI.On("GetBundlePath").Return("/test/testBundlePath", nil)

	p.userStore = getMockUserStoreKV()
	p.instanceStore = p.getMockInstanceStoreKV(1)

	p.otsStore = p.getMockOTSStoreKV()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			link := instancePathWithAdmin(routeUserConnect, "testInstanceID", tc.isAdmin)
			request := httptest.NewRequest(tc.method, fmt.Sprintf(httpOAuth2ConnectURL, link), nil)
			request.Header.Set("Mattermost-User-Id", tc.userID)
			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, request)
			assert.Equal(t, tc.statusCode, w.Result().StatusCode)
		})
	}
}

func TestHttpOAuth2Complete(t *testing.T) {
	tests := map[string]struct {
		method     string
		userID     string
		statusCode int
		isAdmin    bool
		instanceID string
	}{
		"wrong API method": {
			method:     "POST",
			userID:     "connected_user",
			statusCode: http.StatusMethodNotAllowed,
			isAdmin:    false,
			instanceID: correctInstanceID},
		"no instance installed": {
			method:     "GET",
			userID:     "connected_user",
			statusCode: http.StatusInternalServerError,
			isAdmin:    false,
			instanceID: wrongInstanceID},
		"user not connected to confluence will atempt connect without admin access": {
			method:     "GET",
			userID:     "connected_user",
			statusCode: http.StatusOK,
			isAdmin:    false,
			instanceID: correctInstanceID},
		"user not connected to confluence will atempt connect with admin access": {
			method:     "GET",
			userID:     "connected_user",
			statusCode: http.StatusOK,
			isAdmin:    true,
			instanceID: correctInstanceID},
	}

	p := Plugin{
		userStore: getMockUserStoreKV(),
	}
	p.instanceStore = p.getMockInstanceStoreKV(1)
	p.otsStore = p.getMockOTSStoreKV()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()
			mockUser := &model.User{
				Id:       "123",
				Roles:    "system_user",
				Username: "test",
			}

			mockAPI.On("LogError", mockAnythingOfTypeBatch("string", 13)...).Return(nil)

			mockAPI.On("LogDebug", mockAnythingOfTypeBatch("string", 11)...).Return(nil)
			mockAPI.On("GetBundlePath").Return("/test/testBundlePath", nil)

			mockAPI.On("LogError", mockAnythingOfTypeBatch("string", 13)...).Return(nil)

			mockAPI.On("LogDebug", mockAnythingOfTypeBatch("string", 11)...).Return(nil)

			mockAPI.On("GetBundlePath").Return("/test/testBundlePath", nil)

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(mockUser, nil)

			p.SetAPI(mockAPI)

			_, filename, _, _ := runtime.Caller(0)
			templates, err := p.loadTemplates(filepath.Dir(filename) + "/../assets/templates")
			require.NoError(t, err)
			p.templates = templates

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "CompleteOAuth2", func(_ *Plugin, mattermostUserID string, code string, state string, instance Instance, isAdmin bool) (*ConfluenceUser, *model.User, error) {
				return &ConfluenceUser{
					AccountID:   "testAccountID",
					Name:        "test",
					DisplayName: "testName",
				}, mockUser, nil
			})

			link := fmt.Sprintf(httpOAuth2CompleteURL, tc.instanceID)

			if tc.isAdmin {
				link += "_admin"
			}

			request := httptest.NewRequest(tc.method, link, nil)
			request.Header.Set("Mattermost-User-Id", tc.userID)
			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, request)
			assert.Equal(t, tc.statusCode, w.Result().StatusCode)
		})
	}
}

func TestHasPermissionToManageSubscription(t *testing.T) {
	tests := map[string]struct {
		instanceID                                string
		userID                                    string
		channelID                                 string
		plugin                                    *Plugin
		RolesAllowedToEditConfluenceSubscriptions string
		ChannelType                               bool
		PermissionModel                           *model.Permission
		apiCalls                                  func(t *testing.T, instanceID, userID, channelID string, plugin *Plugin)
	}{
		"team admin access": {
			userID:     "connected_user",
			instanceID: correctInstanceID,
			channelID:  "testChannelID",
			RolesAllowedToEditConfluenceSubscriptions: "team_admin",
			PermissionModel: model.PermissionManageTeam,
			ChannelType:     true,
			apiCalls: func(t *testing.T, instanceID, userID, channelID string, plugin *Plugin) {
				err := plugin.HasPermissionToManageSubscription(instanceID, userID, channelID)
				assert.Nil(t, err)
			},
		},

		"channel admin public channel": {
			userID:     "connected_user",
			instanceID: correctInstanceID,
			channelID:  "testChannelID",
			RolesAllowedToEditConfluenceSubscriptions: "channel_admin",
			PermissionModel: model.PermissionManagePublicChannelProperties,
			ChannelType:     true,
			apiCalls: func(t *testing.T, instanceID, userID, channelID string, plugin *Plugin) {
				err := plugin.HasPermissionToManageSubscription(instanceID, userID, channelID)
				assert.Nil(t, err)
			},
		},

		"channel admin private channel": {
			userID:     "connected_user",
			instanceID: correctInstanceID,
			channelID:  "testChannelID",
			RolesAllowedToEditConfluenceSubscriptions: "channel_admin",
			PermissionModel: model.PermissionManagePrivateChannelProperties,
			ChannelType:     false,
			apiCalls: func(t *testing.T, instanceID, userID, channelID string, plugin *Plugin) {
				err := plugin.HasPermissionToManageSubscription(instanceID, userID, channelID)
				assert.Nil(t, err)
			},
		},

		"default access": {
			userID:     "connected_user",
			instanceID: correctInstanceID,
			channelID:  "testChannelID",
			RolesAllowedToEditConfluenceSubscriptions: "",
			PermissionModel: model.PermissionManageSystem,
			ChannelType:     true,
			apiCalls: func(t *testing.T, instanceID, userID, channelID string, plugin *Plugin) {
				err := plugin.HasPermissionToManageSubscription(instanceID, userID, channelID)
				assert.Nil(t, err)
			},
		},
	}

	mockAPI := baseMock()

	p := Plugin{}
	p.SetAPI(mockAPI)
	p.userStore = getMockUserStoreKV()
	p.instanceStore = p.getMockInstanceStoreKV(1)
	p.otsStore = p.getMockOTSStoreKV()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p.conf.RolesAllowedToEditConfluenceSubscriptions = tc.RolesAllowedToEditConfluenceSubscriptions

			mockChannel := &model.Channel{
				Id:   tc.channelID,
				Type: model.ChannelTypeOpen,
			}
			if !tc.ChannelType {
				mockChannel.Type = model.ChannelTypePrivate
			}
			mockAPI.On("GetChannel", tc.channelID).Return(mockChannel, nil)

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "HasPermissionToManageSubscriptionForConfluenceSide", func(_ *Plugin, instanceID, userID string) error {
				return nil
			})

			mockAPI.On("HasPermissionToChannel", mock.AnythingOfType("string"), mock.AnythingOfType("string"), tc.PermissionModel).Return(true)
			mockAPI.On("HasPermissionTo", mock.AnythingOfType("string"), tc.PermissionModel).Return(true)

			tc.apiCalls(t, tc.instanceID, tc.userID, tc.channelID, &p)
		})
	}
}
