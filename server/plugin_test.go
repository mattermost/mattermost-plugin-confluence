package main

import (
	"fmt"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
)

const (
	userID    = "abcdabcdabcdabcd"
	channelID = "testChannelID"
)

func TestExecuteCommand(t *testing.T) {
	p := Plugin{}
	mockAPI := &plugintest.API{}
	p.API = mockAPI
	for name, val := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
		patchAPICalls    func()
	}{
		"empty command ": {
			commandArgs:      &model.CommandArgs{Command: "/confluence", UserId: userID, ChannelId: channelID},
			ephemeralMessage: commonHelpText,
			isAdmin:          false,
		},
		"help command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence help", UserId: userID, ChannelId: channelID},
			ephemeralMessage: commonHelpText,
			isAdmin:          false,
		},
		"unsubscribe command ": {
			commandArgs:      &model.CommandArgs{Command: "/confluence unsubscribe \"abc\"", UserId: userID, ChannelId: channelID},
			ephemeralMessage: fmt.Sprintf(subscriptionNotFound, "abc"),
			isAdmin:          false,
			patchAPICalls: func() {
				monkey.Patch(p.DeleteSubscription, func(channelID, alias, userID string) error {
					return nil
				})
			},
		},
		"unsubscribe command no alias": {
			commandArgs:      &model.CommandArgs{Command: "/confluence unsubscribe", UserId: userID, ChannelId: channelID},
			ephemeralMessage: specifyAlias,
			isAdmin:          false,
		},
		"invalid command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence xyz", UserId: userID, ChannelId: channelID},
			ephemeralMessage: invalidCommand,
			isAdmin:          false,
		},
		"admin restricted": {
			commandArgs:      &model.CommandArgs{Command: "/confluence install \"server\" \"https:\\confluence.test.com\"", UserId: userID, ChannelId: channelID},
			ephemeralMessage: installOnlySystemAdmin,
			isAdmin:          false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()

			monkey.Patch(service.GetSubscriptions, func() (serializer.Subscriptions, error) {
				return subscriptions, nil
			})

			monkey.Patch(service.GetOldSubscriptions, func() ([]serializer.Subscription, error) {
				return nil, nil
			})

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				post := args.Get(1).(*model.Post)
				assert.Equal(t, val.ephemeralMessage, post.Message)
			}).Once().Return(&model.Post{})

			roles := "system_user"
			if val.isAdmin {
				roles += " system_admin"
			}
			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Id: "123", Roles: roles}, nil)
			if val.patchAPICalls != nil {
				val.patchAPICalls()
			}

			res, err := p.ExecuteCommand(&plugin.Context{}, val.commandArgs)
			assert.Nil(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestConfigCommand(t *testing.T) {
	p := Plugin{}
	mockAPI := &plugintest.API{}
	p.API = mockAPI
	for name, test := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
	}{
		"invalid config command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config", UserId: userID, ChannelId: channelID},
			ephemeralMessage: invalidCommand,
			isAdmin:          false,
		},
		"admin restriction on config command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config ", UserId: userID, ChannelId: channelID},
			ephemeralMessage: installOnlySystemAdmin,
			isAdmin:          false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				post := args.Get(1).(*model.Post)
				assert.Equal(t, test.ephemeralMessage, post.Message)
			}).Once().Return(&model.Post{})

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetSiteURL", func(_ *Plugin) string {
				return "https://test.com"
			})

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetPluginURL", func(_ *Plugin) string {
				return "https://test.com/api/v4/actions/dialogs/open"
			})

			monkey.Patch(service.GetOldSubscriptions, func() ([]serializer.Subscription, error) {
				return nil, nil
			})

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(getMockUser(test.isAdmin), nil)

			res, err := p.ExecuteCommand(&plugin.Context{}, test.commandArgs)
			assert.Nil(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestConfigAddCommand(t *testing.T) {
	p := Plugin{}
	mockAPI := &plugintest.API{}
	p.API = mockAPI
	for name, test := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
	}{
		"admin restriction on config add command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config add", UserId: userID, ChannelId: channelID},
			ephemeralMessage: installOnlySystemAdmin,
			isAdmin:          false,
		},
		"config add command success": {
			commandArgs: &model.CommandArgs{Command: "/confluence config \"add https://example.com\"", UserId: userID, ChannelId: channelID},
			isAdmin:     true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				post := args.Get(1).(*model.Post)
				if test.ephemeralMessage != "" {
					assert.Equal(t, test.ephemeralMessage, post.Message)
				}
			}).Once().Return(&model.Post{})

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(getMockUser(test.isAdmin), nil)

			monkey.Patch(service.GetOldSubscriptions, func() ([]serializer.Subscription, error) {
				return nil, nil
			})

			res, err := p.ExecuteCommand(&plugin.Context{}, test.commandArgs)
			assert.Nil(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestConfigListCommand(t *testing.T) {
	p := Plugin{}
	mockAPI := &plugintest.API{}
	p.API = mockAPI
	p.instanceStore = p.getMockInstanceStoreKV(1)
	for name, test := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
		patchAPICalls    func()
	}{
		"admin restriction on config list command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config list", UserId: userID, ChannelId: channelID},
			ephemeralMessage: installOnlySystemAdmin,
			isAdmin:          false,
		},
		"config list command no config saved": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config list", UserId: userID, ChannelId: channelID},
			ephemeralMessage: noSavedConfig,
			isAdmin:          true,
			patchAPICalls: func() {
				monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetConfigKeyList", func(_ *Plugin) ([]string, error) {
					return []string{}, nil
				})
			},
		},
		"config list command success": {
			commandArgs: &model.CommandArgs{Command: "/confluence config list", UserId: userID, ChannelId: channelID},
			isAdmin:     true,
			patchAPICalls: func() {
				monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetConfigKeyList", func(_ *Plugin) ([]string, error) {
					return []string{"https://test.com"}, nil
				})
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				post := args.Get(1).(*model.Post)
				if test.ephemeralMessage != "" {
					assert.Equal(t, test.ephemeralMessage, post.Message)
				}
			}).Once().Return(&model.Post{})

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(getMockUser(test.isAdmin), nil)
			if test.patchAPICalls != nil {
				test.patchAPICalls()
			}

			res, err := p.ExecuteCommand(&plugin.Context{}, test.commandArgs)
			assert.Nil(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestConfigDeleteCommand(t *testing.T) {
	p := Plugin{}
	mockAPI := &plugintest.API{}
	p.API = mockAPI
	p.instanceStore = p.getMockInstanceStoreKV(1)
	for name, test := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
	}{
		"admin restriction on config delete command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config delete", UserId: userID, ChannelId: channelID},
			ephemeralMessage: installOnlySystemAdmin,
			isAdmin:          false,
		},
		"config delete command success": {
			commandArgs:      &model.CommandArgs{Command: "/confluence config delete \"https://test.com\" ", UserId: userID, ChannelId: channelID},
			ephemeralMessage: "Your config is deleted for confluence instance https://test.com",
			isAdmin:          true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			mockAPI := baseMock()

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				post := args.Get(1).(*model.Post)
				assert.Equal(t, test.ephemeralMessage, post.Message)
			}).Once().Return(&model.Post{})

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetSiteURL", func(_ *Plugin) string {
				return "https://test.com"
			})

			monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetPluginURL", func(_ *Plugin) string {
				return "https://test.com/api/v4/actions/dialogs/open"
			})

			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(getMockUser(test.isAdmin), nil)

			res, err := p.ExecuteCommand(&plugin.Context{}, test.commandArgs)
			assert.Nil(t, err)
			assert.NotNil(t, res)
		})
	}
}
