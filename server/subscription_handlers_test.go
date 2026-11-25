package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	subTestEncryptionKey = "0123456789abcdef0123456789abcdef"
	subTestOwnAlias      = "my-alias"
	subTestOwnSpaceKey   = "OWN"
	subTestOwnPageID     = "67890"
)

func seedSubscription(t *testing.T, subs *serializer.Subscriptions, subscriptionType, channelID, alias, key, createdBy string, events []string) {
	t.Helper()
	base := serializer.BaseSubscription{
		Alias:     alias,
		BaseURL:   subTestBaseURL,
		Events:    events,
		ChannelID: channelID,
		Type:      subscriptionType,
		CreatedBy: createdBy,
	}
	var sub serializer.Subscription
	if subscriptionType == serializer.SubscriptionTypeSpace {
		sub = serializer.SpaceSubscription{SpaceKey: key, BaseSubscription: base}
	} else {
		sub = serializer.PageSubscription{PageID: key, BaseSubscription: base}
	}
	require.NoError(t, sub.Add(subs))
}

func subscriptionKey(sub serializer.Subscription) string {
	switch s := sub.(type) {
	case serializer.SpaceSubscription:
		return s.SpaceKey
	case serializer.PageSubscription:
		return s.PageID
	case *serializer.SpaceSubscription:
		return s.SpaceKey
	case *serializer.PageSubscription:
		return s.PageID
	}
	return ""
}

func ownKey(subscriptionType string) string {
	if subscriptionType == serializer.SubscriptionTypeSpace {
		return subTestOwnSpaceKey
	}
	return subTestOwnPageID
}

func sharedKey(subscriptionType string) string {
	if subscriptionType == serializer.SubscriptionTypeSpace {
		return subTestSpaceKey
	}
	return subTestPageID
}

func setBodyKey(body map[string]interface{}, subscriptionType, key string) {
	if subscriptionType == serializer.SubscriptionTypeSpace {
		body["spaceKey"] = key
	} else {
		body["pageID"] = key
	}
}

// setupPerUserAuthSubscriptionRouter wires a plugin configured for per-user Confluence auth
// (Confluence Server 9+) whose caller is a connected, non-admin member of subTestMemberChannel
// only, without channel-admin permissions, and who can read every Confluence space and page.
func setupPerUserAuthSubscriptionRouter(t *testing.T, initial *serializer.Subscriptions) *subscriptionTestEnv {
	t.Helper()

	confluence := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if key, ok := strings.CutPrefix(r.URL.Path, PathSpaceData); ok {
			_ = json.NewEncoder(w).Encode(SpaceResponse{Key: key})
			return
		}
		if strings.HasPrefix(r.URL.Path, PathContentData) {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(confluence.Close)

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
	mockAPI.On("GetConfig").Return(&model.Config{}).Maybe()

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

	mockAPI.On("GetUser", subTestUserID).Return(&model.User{Id: subTestUserID, Roles: model.SystemUserRoleId}, nil).Maybe()
	mockAPI.On("GetChannelMember", subTestMemberChannel, subTestUserID).Return(&model.ChannelMember{ChannelId: subTestMemberChannel, UserId: subTestUserID}, nil).Maybe()
	mockAPI.On("GetChannelMember", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil, model.NewAppError("GetChannelMember", "not_member", nil, "", http.StatusNotFound)).Maybe()
	mockAPI.On("HasPermissionToChannel", mock.Anything, mock.Anything, mock.Anything).Return(false).Maybe()
	mockAPI.On("SendEphemeralPost", mock.Anything, mock.Anything).Return(&model.Post{}).Maybe()

	config.SetConfig(&config.Configuration{
		Secret:                    testWebhookKey,
		EncryptionKey:             subTestEncryptionKey,
		ConfluenceURL:             confluence.URL,
		ServerVersionGreaterthan9: true,
	})

	p := &Plugin{}
	p.SetAPI(mockAPI)
	p.client = pluginapi.NewClient(mockAPI, nil)

	token, err := p.NewEncodedAuthToken(&oauth2.Token{AccessToken: "access-token", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	connection, err := json.Marshal(&types.Connection{
		ConfluenceUser:   types.ConfluenceUser{AccountID: "confluence-account-id"},
		OAuth2Token:      token,
		MattermostUserID: subTestUserID,
	})
	require.NoError(t, err)
	env.kv[confluence.URL+"_"+subTestUserID] = connection

	env.router = p.InitAPI()
	return env
}

func TestSubscriptionChannelBindingPerUserAuth(t *testing.T) {
	ownerEvents := []string{serializer.PageCreatedEvent, serializer.PageUpdatedEvent}
	callerOriginalEvents := []string{serializer.CommentCreatedEvent}
	requestEvents := []string{serializer.PageRemovedEvent}

	for name, tc := range map[string]struct {
		method           string
		subscriptionType string
	}{
		"save space subscription": {method: http.MethodPost, subscriptionType: serializer.SubscriptionTypeSpace},
		"save page subscription":  {method: http.MethodPost, subscriptionType: serializer.SubscriptionTypePage},
		"edit space subscription": {method: http.MethodPut, subscriptionType: serializer.SubscriptionTypeSpace},
		"edit page subscription":  {method: http.MethodPut, subscriptionType: serializer.SubscriptionTypePage},
	} {
		t.Run(name, func(t *testing.T) {
			var initial *serializer.Subscriptions
			if tc.method == http.MethodPut {
				initial = serializer.NewSubscriptions()
				addExistingSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, subTestUserID, callerOriginalEvents)
				addExistingSubscription(t, initial, tc.subscriptionType, subTestPrivateChannel, subTestOwnerID, ownerEvents)
			}
			env := setupPerUserAuthSubscriptionRouter(t, initial)

			body := subscriptionBody(tc.subscriptionType, subTestPrivateChannel, requestEvents)
			if tc.method == http.MethodPut {
				body["oldAlias"] = subTestAlias
			}
			w := env.do(t, tc.method, subTestMemberChannel, tc.subscriptionType, body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			subs := env.subscriptions(t)

			stored, ok := subs.ByChannelID[subTestMemberChannel][subTestAlias]
			require.True(t, ok, "the request must apply to the URL channel")
			assert.Equal(t, subTestUserID, stored.GetCreatedBy())
			assert.Equal(t, requestEvents, subscriptionEvents(stored))
			assert.Equal(t, requestEvents, urlIndex(subs, tc.subscriptionType)[subTestMemberChannel])

			if tc.method == http.MethodPost {
				assert.Empty(t, subs.ByChannelID[subTestPrivateChannel])
				assert.NotContains(t, urlIndex(subs, tc.subscriptionType), subTestPrivateChannel)
				return
			}

			other, ok := subs.ByChannelID[subTestPrivateChannel][subTestAlias]
			require.True(t, ok, "subscription in another channel must not be removed")
			assert.Equal(t, subTestOwnerID, other.GetCreatedBy())
			assert.Equal(t, ownerEvents, subscriptionEvents(other))
			assert.Equal(t, ownerEvents, urlIndex(subs, tc.subscriptionType)[subTestPrivateChannel])
		})
	}
}

func TestHandleSaveSubscriptionDoesNotOverwriteExistingSubscription(t *testing.T) {
	ownerEvents := []string{serializer.PageCreatedEvent, serializer.PageUpdatedEvent}

	for name, tc := range map[string]struct {
		subscriptionType string
		reuseAlias       bool
		reuseKey         bool
	}{
		"space subscription reusing another user's alias":     {subscriptionType: serializer.SubscriptionTypeSpace, reuseAlias: true},
		"page subscription reusing another user's alias":      {subscriptionType: serializer.SubscriptionTypePage, reuseAlias: true},
		"space subscription reusing another user's space key": {subscriptionType: serializer.SubscriptionTypeSpace, reuseKey: true},
		"page subscription reusing another user's page ID":    {subscriptionType: serializer.SubscriptionTypePage, reuseKey: true},
	} {
		t.Run(name, func(t *testing.T) {
			initial := serializer.NewSubscriptions()
			seedSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, subTestAlias, sharedKey(tc.subscriptionType), subTestOwnerID, ownerEvents)
			env := setupPerUserAuthSubscriptionRouter(t, initial)

			body := subscriptionBody(tc.subscriptionType, subTestMemberChannel, []string{serializer.PageRemovedEvent})
			if !tc.reuseAlias {
				body["alias"] = subTestOwnAlias
			}
			if !tc.reuseKey {
				setBodyKey(body, tc.subscriptionType, ownKey(tc.subscriptionType))
			}
			w := env.do(t, http.MethodPost, subTestMemberChannel, tc.subscriptionType, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			subs := env.subscriptions(t)
			other, ok := subs.ByChannelID[subTestMemberChannel][subTestAlias]
			require.True(t, ok)
			assert.Equal(t, subTestOwnerID, other.GetCreatedBy())
			assert.Equal(t, sharedKey(tc.subscriptionType), subscriptionKey(other))
			assert.Equal(t, ownerEvents, subscriptionEvents(other))
			assert.Equal(t, ownerEvents, urlIndex(subs, tc.subscriptionType)[subTestMemberChannel])
		})
	}
}

// The caller owns one subscription in the URL channel and another user owns a second one there;
// the caller is not a channel admin, so no edit may alter the other user's subscription.
func TestHandleEditSubscriptionCannotAlterAnotherUsersSubscription(t *testing.T) {
	ownerEvents := []string{serializer.PageCreatedEvent, serializer.PageUpdatedEvent}
	callerEvents := []string{serializer.CommentCreatedEvent}
	editedEvents := []string{serializer.PageRemovedEvent}

	// The handler resolves oldAlias case-insensitively over a Go map, so a single request only
	// sometimes resolves to the caller's subscription; repeat to make the outcome deterministic.
	const caseConfusionAttempts = 32

	type editCase struct {
		subscriptionType string
		callerAlias      string
		oldAlias         string
		newAlias         string
		retargetToShared bool
		attempts         int
	}
	cases := map[string]editCase{}
	for _, subscriptionType := range []string{serializer.SubscriptionTypeSpace, serializer.SubscriptionTypePage} {
		cases[subscriptionType+" renamed onto another user's alias"] = editCase{
			subscriptionType: subscriptionType,
			callerAlias:      subTestOwnAlias,
			oldAlias:         subTestOwnAlias,
			newAlias:         subTestAlias,
		}
		cases[subscriptionType+" retargeted onto another user's subscribed key"] = editCase{
			subscriptionType: subscriptionType,
			callerAlias:      subTestOwnAlias,
			oldAlias:         subTestOwnAlias,
			newAlias:         subTestOwnAlias,
			retargetToShared: true,
		}
		cases[subscriptionType+" edited via another user's alias differing only in case"] = editCase{
			subscriptionType: subscriptionType,
			callerAlias:      strings.ToUpper(subTestAlias),
			oldAlias:         subTestAlias,
			newAlias:         subTestAlias,
			retargetToShared: true,
			attempts:         caseConfusionAttempts,
		}
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			attempts := max(tc.attempts, 1)
			for range attempts {
				initial := serializer.NewSubscriptions()
				seedSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, tc.callerAlias, ownKey(tc.subscriptionType), subTestUserID, callerEvents)
				seedSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, subTestAlias, sharedKey(tc.subscriptionType), subTestOwnerID, ownerEvents)
				env := setupPerUserAuthSubscriptionRouter(t, initial)

				body := subscriptionBody(tc.subscriptionType, subTestMemberChannel, editedEvents)
				body["alias"] = tc.newAlias
				body["oldAlias"] = tc.oldAlias
				if !tc.retargetToShared {
					setBodyKey(body, tc.subscriptionType, ownKey(tc.subscriptionType))
				}
				w := env.do(t, http.MethodPut, subTestMemberChannel, tc.subscriptionType, body)

				subs := env.subscriptions(t)
				other, ok := subs.ByChannelID[subTestMemberChannel][subTestAlias]
				require.True(t, ok, "another user's subscription must not be removed (status %d)", w.Code)
				require.Equal(t, subTestOwnerID, other.GetCreatedBy(), "another user's subscription must keep its creator (status %d)", w.Code)
				require.Equal(t, sharedKey(tc.subscriptionType), subscriptionKey(other), "another user's subscription must keep its target (status %d)", w.Code)
				require.Equal(t, ownerEvents, subscriptionEvents(other), "another user's subscription must keep its events (status %d)", w.Code)
				require.Equal(t, ownerEvents, urlIndex(subs, tc.subscriptionType)[subTestMemberChannel], "notification index for another user's subscription must keep its events (status %d)", w.Code)
			}
		})
	}
}
