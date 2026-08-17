package main

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const testConfluenceURL = "https://test.atlassian.net"

func marshalConnection(t *testing.T, connection *types.Connection) []byte {
	t.Helper()
	b, err := json.Marshal(connection)
	require.NoError(t, err)
	return b
}

func TestCheckSubscriptionAccess(t *testing.T) {
	connected := marshalConnection(t, &types.Connection{ConfluenceUser: types.ConfluenceUser{AccountID: "confluence-account-id"}})
	connectedWithoutAccount := marshalConnection(t, &types.Connection{})

	cloudConfig := &config.Configuration{ConfluenceURL: testConfluenceURL, IsCloud: true}
	serverV9Config := &config.Configuration{ConfluenceURL: testConfluenceURL, ServerVersionGreaterthan9: true}
	legacyServerConfig := &config.Configuration{ConfluenceURL: testConfluenceURL}

	for name, tc := range map[string]struct {
		pluginConfig    *config.Configuration
		roles           string
		connection      []byte
		expectedAllowed bool
		expectedReason  string
	}{
		"cloud, system admin without a connection": {
			pluginConfig:    cloudConfig,
			roles:           model.SystemAdminRoleId,
			expectedAllowed: true,
		},
		"cloud, connected non-admin": {
			pluginConfig:    cloudConfig,
			roles:           model.SystemUserRoleId,
			connection:      connected,
			expectedAllowed: true,
		},
		"cloud, disconnected non-admin": {
			pluginConfig:   cloudConfig,
			roles:          model.SystemUserRoleId,
			expectedReason: subscribeDeniedNotConnected,
		},
		"cloud, non-admin whose connection has no Confluence account": {
			pluginConfig:   cloudConfig,
			roles:          model.SystemUserRoleId,
			connection:     connectedWithoutAccount,
			expectedReason: subscribeDeniedNotConnected,
		},
		"server 9+, connected non-admin": {
			pluginConfig:    serverV9Config,
			roles:           model.SystemUserRoleId,
			connection:      connected,
			expectedAllowed: true,
		},
		"server below 9, connected non-admin stays admin-only": {
			pluginConfig:   legacyServerConfig,
			roles:          model.SystemUserRoleId,
			connection:     connected,
			expectedReason: subscribeDeniedAdminOnly,
		},
		"server below 9, system admin": {
			pluginConfig:    legacyServerConfig,
			roles:           model.SystemAdminRoleId,
			expectedAllowed: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			config.Mattermost = mockAPI
			config.SetConfig(tc.pluginConfig)

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Roles: tc.roles}, nil)
			mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(tc.connection, nil)

			access := (&Plugin{}).checkSubscriptionAccess("user-id")

			assert.Equal(t, tc.expectedAllowed, access.Allowed)
			assert.Equal(t, tc.expectedReason, access.Reason)
			if !tc.expectedAllowed {
				assert.NotEmpty(t, access.Message)
			}
		})
	}
}
