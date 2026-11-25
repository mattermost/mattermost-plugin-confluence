package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

func addExistingSubscription(t *testing.T, subs *serializer.Subscriptions, subscriptionType, channelID, createdBy string, events []string) {
	t.Helper()
	base := serializer.BaseSubscription{
		Alias:     subTestAlias,
		BaseURL:   subTestBaseURL,
		Events:    events,
		ChannelID: channelID,
		Type:      subscriptionType,
		CreatedBy: createdBy,
	}
	var sub serializer.Subscription
	if subscriptionType == serializer.SubscriptionTypeSpace {
		sub = serializer.SpaceSubscription{SpaceKey: subTestSpaceKey, BaseSubscription: base}
	} else {
		sub = serializer.PageSubscription{PageID: subTestPageID, BaseSubscription: base}
	}
	require.NoError(t, sub.Add(subs))
}

func subscriptionEvents(sub serializer.Subscription) []string {
	switch s := sub.(type) {
	case serializer.SpaceSubscription:
		return s.Events
	case serializer.PageSubscription:
		return s.Events
	case *serializer.SpaceSubscription:
		return s.Events
	case *serializer.PageSubscription:
		return s.Events
	}
	return nil
}

// grantChannelAdmin makes the caller a channel admin of subTestMemberChannel. The expectation is
// moved ahead of the catch-all HasPermissionToChannel expectation registered by the router setup.
func grantChannelAdmin(t *testing.T) {
	t.Helper()
	api, ok := config.Mattermost.(*plugintest.API)
	require.True(t, ok)
	api.On("HasPermissionToChannel", subTestUserID, subTestMemberChannel, model.PermissionManageChannelRoles).Return(true).Maybe()
	last := len(api.ExpectedCalls) - 1
	api.ExpectedCalls = append([]*mock.Call{api.ExpectedCalls[last]}, api.ExpectedCalls[:last]...)
}

func subscriptionURLIndex(subs *serializer.Subscriptions, subscriptionType, key string) serializer.StringArrayMap {
	if subscriptionType == serializer.SubscriptionTypeSpace {
		return subs.ByURLSpaceKey[store.GetURLSpaceKeyCombinationKey(subTestBaseURL, key)]
	}
	return subs.ByURLPageID[store.GetURLPageIDCombinationKey(subTestBaseURL, key)]
}

func TestHandleEditSubscriptionLegitimateFlows(t *testing.T) {
	originalEvents := []string{serializer.CommentCreatedEvent}
	editedEvents := []string{serializer.PageRemovedEvent}

	type editCase struct {
		subscriptionType string
		storedAlias      string
		storedCreatedBy  string
		channelAdmin     bool
		newAlias         string
		retarget         bool
		expectStatus     int
	}
	retargetKey := map[string]string{
		serializer.SubscriptionTypeSpace: "RETARGET",
		serializer.SubscriptionTypePage:  "55555",
	}

	cases := map[string]editCase{}
	for _, subscriptionType := range []string{serializer.SubscriptionTypeSpace, serializer.SubscriptionTypePage} {
		cases[subscriptionType+" creator renames and retargets own subscription"] = editCase{
			subscriptionType: subscriptionType,
			storedAlias:      subTestOwnAlias,
			storedCreatedBy:  subTestUserID,
			newAlias:         "renamed-alias",
			retarget:         true,
			expectStatus:     http.StatusOK,
		}
		cases[subscriptionType+" creator changes only the case of own alias"] = editCase{
			subscriptionType: subscriptionType,
			storedAlias:      "My-Alias",
			storedCreatedBy:  subTestUserID,
			newAlias:         "my-alias",
			expectStatus:     http.StatusOK,
		}
		cases[subscriptionType+" channel admin renames and retargets another user's subscription"] = editCase{
			subscriptionType: subscriptionType,
			storedAlias:      subTestAlias,
			storedCreatedBy:  subTestOwnerID,
			channelAdmin:     true,
			newAlias:         "renamed-alias",
			retarget:         true,
			expectStatus:     http.StatusOK,
		}
		cases[subscriptionType+" channel admin edits legacy subscription without a creator"] = editCase{
			subscriptionType: subscriptionType,
			storedAlias:      subTestAlias,
			channelAdmin:     true,
			newAlias:         subTestAlias,
			expectStatus:     http.StatusOK,
		}
		cases[subscriptionType+" non-admin cannot edit legacy subscription without a creator"] = editCase{
			subscriptionType: subscriptionType,
			storedAlias:      subTestAlias,
			newAlias:         subTestAlias,
			expectStatus:     http.StatusForbidden,
		}
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			originalKey := ownKey(tc.subscriptionType)
			initial := serializer.NewSubscriptions()
			seedSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, tc.storedAlias, originalKey, tc.storedCreatedBy, originalEvents)
			env := setupPerUserAuthSubscriptionRouter(t, initial)
			if tc.channelAdmin {
				grantChannelAdmin(t)
			}

			newKey := originalKey
			if tc.retarget {
				newKey = retargetKey[tc.subscriptionType]
			}
			body := subscriptionBody(tc.subscriptionType, subTestMemberChannel, editedEvents)
			body["alias"] = tc.newAlias
			body["oldAlias"] = tc.storedAlias
			setBodyKey(body, tc.subscriptionType, newKey)

			w := env.do(t, http.MethodPut, subTestMemberChannel, tc.subscriptionType, body)
			require.Equal(t, tc.expectStatus, w.Code, w.Body.String())

			subs := env.subscriptions(t)
			channelSubs := subs.ByChannelID[subTestMemberChannel]

			if tc.expectStatus != http.StatusOK {
				stored, ok := channelSubs[tc.storedAlias]
				require.True(t, ok)
				assert.Equal(t, originalEvents, subscriptionEvents(stored))
				assert.Equal(t, originalEvents, subscriptionURLIndex(subs, tc.subscriptionType, originalKey)[subTestMemberChannel])
				return
			}

			require.Len(t, channelSubs, 1, "the edit must replace the subscription, not duplicate it")
			edited, ok := channelSubs[tc.newAlias]
			require.True(t, ok, "the subscription must be stored under the new alias")
			assert.Equal(t, tc.newAlias, edited.GetAlias())
			assert.Equal(t, tc.storedCreatedBy, edited.GetCreatedBy(), "editing must keep the original creator")
			assert.Equal(t, newKey, subscriptionKey(edited))
			assert.Equal(t, editedEvents, subscriptionEvents(edited))
			assert.Equal(t, editedEvents, subscriptionURLIndex(subs, tc.subscriptionType, newKey)[subTestMemberChannel])
			if tc.retarget {
				assert.NotContains(t, subscriptionURLIndex(subs, tc.subscriptionType, originalKey), subTestMemberChannel, "the old target must no longer notify the channel")
			}
		})
	}
}

func TestHandleEditSubscriptionChannelBinding(t *testing.T) {
	ownerEvents := []string{serializer.PageCreatedEvent, serializer.PageUpdatedEvent}
	callerOriginalEvents := []string{serializer.CommentCreatedEvent}
	editedEvents := []string{serializer.PageRemovedEvent}

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
			initial := serializer.NewSubscriptions()
			addExistingSubscription(t, initial, tc.subscriptionType, subTestMemberChannel, subTestUserID, callerOriginalEvents)
			addExistingSubscription(t, initial, tc.subscriptionType, subTestPrivateChannel, subTestOwnerID, ownerEvents)
			env := setupSubscriptionRouter(t, initial)

			body := subscriptionBody(tc.subscriptionType, tc.bodyChannelID, editedEvents)
			body["oldAlias"] = subTestAlias
			w := env.do(t, http.MethodPut, subTestMemberChannel, tc.subscriptionType, body)

			subs := env.subscriptions(t)

			other, ok := subs.ByChannelID[subTestPrivateChannel][subTestAlias]
			require.True(t, ok, "subscription in another channel must not be removed")
			assert.Equal(t, subTestOwnerID, other.GetCreatedBy(), "subscription in another channel must keep its creator")
			assert.Equal(t, ownerEvents, subscriptionEvents(other), "subscription in another channel must keep its events")
			assert.Equal(t, ownerEvents, urlIndex(subs, tc.subscriptionType)[subTestPrivateChannel], "URL index for another channel must keep its events")

			if tc.expectSuccess {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			}

			own, ok := subs.ByChannelID[subTestMemberChannel][subTestAlias]
			require.True(t, ok, "subscription in the URL channel must still exist")
			assert.Equal(t, subTestUserID, own.GetCreatedBy())

			if w.Code == http.StatusOK {
				assert.Equal(t, editedEvents, subscriptionEvents(own), "successful edit must apply to the URL channel subscription")
				assert.Equal(t, editedEvents, urlIndex(subs, tc.subscriptionType)[subTestMemberChannel])
			} else {
				assert.GreaterOrEqual(t, w.Code, 400)
				assert.Less(t, w.Code, 500)
				assert.Equal(t, callerOriginalEvents, subscriptionEvents(own))
			}
		})
	}
}
