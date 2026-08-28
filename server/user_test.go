package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const testConfluenceURL = "https://test.atlassian.net"

func marshalConnection(t *testing.T, connection *types.Connection) []byte {
	t.Helper()
	b, err := json.Marshal(connection)
	require.NoError(t, err)
	return b
}

func TestClassifyConfluenceAccessError(t *testing.T) {
	for name, tc := range map[string]struct {
		err                error
		expectedStatusCode int
	}{
		"not found is treated as hidden from the user": {
			err:                &APIError{StatusCode: http.StatusNotFound, Path: "/space/MM"},
			expectedStatusCode: http.StatusForbidden,
		},
		"forbidden denies access": {
			err:                &APIError{StatusCode: http.StatusForbidden, Path: "/space/MM"},
			expectedStatusCode: http.StatusForbidden,
		},
		"unauthorized asks the user to reconnect": {
			err:                &APIError{StatusCode: http.StatusUnauthorized, Path: "/space/MM"},
			expectedStatusCode: http.StatusUnauthorized,
		},
		"rate limit is retryable, not a denial": {
			err:                &APIError{StatusCode: http.StatusTooManyRequests, Path: "/space/MM"},
			expectedStatusCode: http.StatusTooManyRequests,
		},
		"upstream outage is not a denial": {
			err:                &APIError{StatusCode: http.StatusServiceUnavailable, Path: "/space/MM"},
			expectedStatusCode: http.StatusBadGateway,
		},
		"wrapped status is still recognized": {
			err:                errors.Wrap(&APIError{StatusCode: http.StatusBadGateway, Path: "/space/MM"}, "upstream failed"),
			expectedStatusCode: http.StatusBadGateway,
		},
		"transport failure without a response is not a denial": {
			err:                errors.New("dial tcp: connection refused"),
			expectedStatusCode: http.StatusBadGateway,
		},
	} {
		t.Run(name, func(t *testing.T) {
			statusCode, err := classifyConfluenceAccessError(tc.err, "space")

			assert.Equal(t, tc.expectedStatusCode, statusCode)
			assert.Error(t, err)
		})
	}
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

func TestCanManageSubscription(t *testing.T) {
	subscriptionCreatedBy := func(userID string) serializer.Subscription {
		return serializer.SpaceSubscription{
			SpaceKey:         "MM",
			BaseSubscription: serializer.BaseSubscription{Alias: "test", CreatedBy: userID},
		}
	}

	for name, tc := range map[string]struct {
		subscription   serializer.Subscription
		isChannelAdmin bool
		expected       bool
	}{
		"creator manages their own subscription": {
			subscription: subscriptionCreatedBy("user-id"),
			expected:     true,
		},
		"member cannot manage someone else's subscription": {
			subscription: subscriptionCreatedBy("another-user-id"),
			expected:     false,
		},
		"member cannot manage a subscription without a creator": {
			subscription: subscriptionCreatedBy(""),
			expected:     false,
		},
		"channel admin manages someone else's subscription": {
			subscription:   subscriptionCreatedBy("another-user-id"),
			isChannelAdmin: true,
			expected:       true,
		},
		"channel admin manages a subscription without a creator": {
			subscription:   subscriptionCreatedBy(""),
			isChannelAdmin: true,
			expected:       true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			mockAPI.On("HasPermissionToChannel", "user-id", "channel-id", model.PermissionManageChannelRoles).Return(tc.isChannelAdmin)

			p := &Plugin{}
			p.API = mockAPI

			assert.Equal(t, tc.expected, p.canManageSubscription("user-id", "channel-id", tc.subscription))
		})
	}
}
