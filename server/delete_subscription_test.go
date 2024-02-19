package main

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	storePackage "github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const (
	mockAlias                      = "test"
	mockBaseURL                    = "https://test.com"
	mockChannelID                  = "testChannelID"
	mockUserIDWithoutNotifications = "2"
)

type mockUserStoreKV struct {
	mockUserStore
	connections map[types.ID]*Connection
	users       map[types.ID]*User
}

type mockInstanceStoreKV struct {
	mockInstanceStore
	kv *sync.Map
	*Instances
	*Plugin
}

type mockOTSStoreKV struct {
	mockOTSStore
	connections map[string]string
}

func getMockUserStoreKV() mockUserStoreKV {
	newuser := func(id types.ID) *User {
		u := NewUser(id)
		u.ConnectedInstances.Set(testInstance1.Common())
		return u
	}

	connection := Connection{
		ConfluenceUser: ConfluenceUser{
			AccountID: "test",
		},
	}

	return mockUserStoreKV{
		users: map[types.ID]*User{
			"connected_user":               newuser("connected_user"),
			"admin":                        newuser("admin"),
			mockUserIDWithoutNotifications: newuser(mockUserIDWithoutNotifications),
		},
		connections: map[types.ID]*Connection{
			mockUserIDWithoutNotifications: &connection,
			"connected_user":               &connection,
			"admin":                        &connection,
		},
	}
}

func (p *Plugin) getMockInstanceStoreKV(n int) *mockInstanceStoreKV {
	kv := sync.Map{}
	instances := NewInstances()

	if n > 2 || n == 0 {
		return &mockInstanceStoreKV{
			kv:        &kv,
			Instances: instances,
			Plugin:    p,
		}
	}

	for i, testInstance := range []*testInstance{testInstance1, testInstance2} {
		if i > n {
			break
		}
		instance := *testInstance
		instance.Plugin = p
		instances.Set(instance.Common())
		kv.Store(instance.GetID(), &instance)
	}

	return &mockInstanceStoreKV{
		kv:        &kv,
		Instances: instances,
		Plugin:    p,
	}
}

func (p *Plugin) getMockOTSStoreKV() *mockOTSStoreKV {
	state := "teststate_admin"
	return &mockOTSStoreKV{
		connections: map[string]string{
			hashkey(prefixOneTimeSecret, state): state,
		},
	}
}

var subscriptions = serializer.Subscriptions{
	ByChannelID: map[string]serializer.StringSubscription{
		"testChannelID": {
			"test": &serializer.SpaceSubscription{
				SpaceKey: "TS",
				UserID:   "testUserID",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     mockAlias,
					BaseURL:   mockBaseURL,
					ChannelID: mockChannelID,
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
		},
		"testChannelID3": {
			"test": &serializer.PageSubscription{
				PageID: "1234",
				UserID: "testUserID",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     mockAlias,
					BaseURL:   mockBaseURL,
					ChannelID: mockChannelID,
					Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				},
			},
		},
	},

	ByURLSpaceKey: map[string]serializer.StringStringArrayMap{
		"testKey": {
			"testChannelID": {
				"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
	},
	ByURLPageID: map[string]serializer.StringStringArrayMap{
		"testKey1": {
			"testChannelID3": {
				"testUserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
	},
}

func TestDeleteSubscription(t *testing.T) {
	p := &Plugin{}
	mockInstanceStore := p.getMockInstanceStoreKV(0)
	mockUserStore := getMockUserStoreKV()
	p.instanceStore = mockInstanceStore
	p.userStore = mockUserStore
	p.API = baseMock()
	for name, val := range map[string]struct {
		channelID string
		alias     string
		userID    string
		plugin    *Plugin
		apiCalls  func(t *testing.T, channelID, alias, userID string, plugin *Plugin)
	}{
		"space subscription delete success": {
			channelID: mockChannelID,
			alias:     mockAlias,
			userID:    "testUserID",
			plugin:    p,
			apiCalls: func(t *testing.T, channelID, alias, userID string, plugin *Plugin) {
				err := p.DeleteSubscription(channelID, alias, userID)
				assert.Nil(t, err)
			},
		},
		"page subscription delete success": {
			channelID: "testChannelID3",
			alias:     mockAlias,
			userID:    "testUserID",
			plugin:    p,
			apiCalls: func(t *testing.T, channelID, alias, userID string, plugin *Plugin) {
				err := p.DeleteSubscription(channelID, alias, userID)
				assert.Nil(t, err)
			},
		},
		"subscription not found with alias": {
			channelID: "testtestesttest",
			alias:     "test1",
			userID:    "testUserID",
			plugin:    p,
			apiCalls: func(t *testing.T, channelID, alias, userID string, plugin *Plugin) {
				err := p.DeleteSubscription(channelID, alias, userID)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
		"no subscription for the channel": {
			channelID: "testtestesttesx",
			alias:     "test1",
			userID:    "testUserID",
			plugin:    p,
			apiCalls: func(t *testing.T, channelID, alias, userID string, plugin *Plugin) {
				err := p.DeleteSubscription(channelID, alias, userID)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			monkey.Patch(service.GetSubscriptions, func() (serializer.Subscriptions, error) {
				return subscriptions, nil
			})

			monkey.Patch(p.getInstanceFromURL, func(confluenceURL string) (Instance, error) {
				return testInstance1, nil
			})

			monkey.Patch(service.GetSubscriptionFromURL, func(URL, userID string) (int, error) {
				return 0, nil
			})

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "HasPermissionToManageSubscription", func(_ *Plugin, instanceID, userID, channelID string) error {
				return nil
			})

			monkey.Patch(storePackage.AtomicModify, func(key string, modify func(initialValue []byte) ([]byte, error)) error {
				return nil
			})

			val.apiCalls(t, val.channelID, val.alias, val.userID, val.plugin)
		})
	}
}
