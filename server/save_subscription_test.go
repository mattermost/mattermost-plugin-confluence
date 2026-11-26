package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

const (
	subTestUserID         = "calleruserid000000000000000"
	subTestOwnerID        = "owneruserid000000000000000"
	subTestMemberChannel  = "memberchannelaaaaaaaaaaaaa"
	subTestPrivateChannel = "privatechannelbbbbbbbbbbbb"
	subTestBaseURL        = "https://confluence.example.com"
	subTestSpaceKey       = "SPACE"
	subTestPageID         = "12345"
	subTestAlias          = "shared-alias"
)

type subscriptionTestEnv struct {
	router http.Handler
	mu     sync.Mutex
	kv     map[string][]byte
}

// setupSubscriptionRouter wires a plugin whose caller is a system admin who is a member of
// subTestMemberChannel only, backed by an in-memory KV store.
func setupSubscriptionRouter(t *testing.T, initial *serializer.Subscriptions) *subscriptionTestEnv {
	t.Helper()

	env := &subscriptionTestEnv{kv: map[string][]byte{}}
	if initial != nil {
		data, err := json.Marshal(initial)
		require.NoError(t, err)
		env.kv[store.GetSubscriptionKey()] = data
	}

	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	allowLogs(mockAPI)
	mockAPI.On("GetBundlePath").Return(t.TempDir(), nil).Maybe()

	mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(func(key string) []byte {
		env.mu.Lock()
		defer env.mu.Unlock()
		return env.kv[key]
	}, nil).Maybe()
	mockAPI.On("KVCompareAndSet", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return(
		func(key string, _, newValue []byte) bool {
			env.mu.Lock()
			defer env.mu.Unlock()
			env.kv[key] = newValue
			return true
		}, nil).Maybe()

	mockAPI.On("GetUser", subTestUserID).Return(&model.User{Id: subTestUserID, Roles: "system_user system_admin"}, nil).Maybe()
	mockAPI.On("GetChannelMember", subTestMemberChannel, subTestUserID).Return(&model.ChannelMember{ChannelId: subTestMemberChannel, UserId: subTestUserID}, nil).Maybe()
	mockAPI.On("GetChannelMember", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil, model.NewAppError("GetChannelMember", "not_member", nil, "", http.StatusNotFound)).Maybe()
	mockAPI.On("HasPermissionToChannel", mock.Anything, mock.Anything, mock.Anything).Return(false).Maybe()
	mockAPI.On("SendEphemeralPost", mock.Anything, mock.Anything).Return(&model.Post{}).Maybe()

	config.SetConfig(&config.Configuration{
		Secret:        testWebhookKey,
		ConfluenceURL: subTestBaseURL,
	})

	p := &Plugin{}
	p.SetAPI(mockAPI)
	p.client = pluginapi.NewClient(mockAPI, nil)

	env.router = p.InitAPI()
	return env
}

func (env *subscriptionTestEnv) do(t *testing.T, method, urlChannelID, subscriptionType string, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(method, "/api/v1/"+urlChannelID+"/subscription/"+subscriptionType, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(config.HeaderMattermostUserID, subTestUserID)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func (env *subscriptionTestEnv) subscriptions(t *testing.T) *serializer.Subscriptions {
	t.Helper()
	env.mu.Lock()
	data := env.kv[store.GetSubscriptionKey()]
	env.mu.Unlock()

	subs, err := serializer.SubscriptionsFromJSON(data)
	require.NoError(t, err)
	subs.EnsureDefaults()
	return subs
}

func subscriptionBody(subscriptionType, channelID string, events []string) map[string]interface{} {
	body := map[string]interface{}{
		"alias":            subTestAlias,
		"baseURL":          subTestBaseURL,
		"events":           events,
		"subscriptionType": subscriptionType,
	}
	if channelID != "" {
		body["channelID"] = channelID
	}
	if subscriptionType == serializer.SubscriptionTypeSpace {
		body["spaceKey"] = subTestSpaceKey
	} else {
		body["pageID"] = subTestPageID
	}
	return body
}

func urlIndex(subs *serializer.Subscriptions, subscriptionType string) serializer.StringArrayMap {
	if subscriptionType == serializer.SubscriptionTypeSpace {
		return subs.ByURLSpaceKey[store.GetURLSpaceKeyCombinationKey(subTestBaseURL, subTestSpaceKey)]
	}
	return subs.ByURLPageID[store.GetURLPageIDCombinationKey(subTestBaseURL, subTestPageID)]
}

func TestHandleSaveSubscriptionChannelBinding(t *testing.T) {
	for name, tc := range map[string]struct {
		subscriptionType string
		bodyChannelID    string
		expectSuccess    bool
	}{
		"space subscription with body channel of another channel": {
			subscriptionType: serializer.SubscriptionTypeSpace,
			bodyChannelID:    subTestPrivateChannel,
		},
		"page subscription with body channel of another channel": {
			subscriptionType: serializer.SubscriptionTypePage,
			bodyChannelID:    subTestPrivateChannel,
		},
		"space subscription with body channel matching URL channel": {
			subscriptionType: serializer.SubscriptionTypeSpace,
			bodyChannelID:    subTestMemberChannel,
			expectSuccess:    true,
		},
		"page subscription with body channel matching URL channel": {
			subscriptionType: serializer.SubscriptionTypePage,
			bodyChannelID:    subTestMemberChannel,
			expectSuccess:    true,
		},
		"space subscription with body channel omitted": {
			subscriptionType: serializer.SubscriptionTypeSpace,
			bodyChannelID:    "",
			expectSuccess:    true,
		},
		"page subscription with body channel omitted": {
			subscriptionType: serializer.SubscriptionTypePage,
			bodyChannelID:    "",
			expectSuccess:    true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := setupSubscriptionRouter(t, nil)

			body := subscriptionBody(tc.subscriptionType, tc.bodyChannelID, []string{serializer.PageCreatedEvent})
			w := env.do(t, http.MethodPost, subTestMemberChannel, tc.subscriptionType, body)

			subs := env.subscriptions(t)

			assert.Empty(t, subs.ByChannelID[subTestPrivateChannel], "no subscription may be stored under a channel other than the authorized URL channel")
			assert.NotContains(t, urlIndex(subs, tc.subscriptionType), subTestPrivateChannel, "no URL index entry may point at a channel other than the authorized URL channel")

			if tc.expectSuccess {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			}

			if w.Code == http.StatusOK {
				stored, ok := subs.ByChannelID[subTestMemberChannel][subTestAlias]
				require.True(t, ok, "successful save must store the subscription under the URL channel")
				assert.Equal(t, subTestUserID, stored.GetCreatedBy())
				assert.Contains(t, urlIndex(subs, tc.subscriptionType), subTestMemberChannel)
			} else {
				assert.GreaterOrEqual(t, w.Code, 400)
				assert.Less(t, w.Code, 500)
			}
		})
	}
}
