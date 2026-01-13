package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func baseMock() *plugintest.API {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI

	return mockAPI
}

func TestExecuteCommand(t *testing.T) {
	p := Plugin{}

	// TODO: Add the testcases for unsubscribe and other commands
	for name, val := range map[string]struct {
		commandArgs      *model.CommandArgs
		ephemeralMessage string
		isAdmin          bool
		patchAPICalls    func()
	}{
		"invalid command": {
			commandArgs:      &model.CommandArgs{Command: "/confluence xyz", UserId: "abcdabcdabcdabcd", ChannelId: "testtesttesttest"},
			ephemeralMessage: invalidCommand,
			isAdmin:          true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockAPI := baseMock()

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

func TestGenerateRandomKey(t *testing.T) {
	t.Run("generates key of correct length", func(t *testing.T) {
		key, err := generateRandomKey(32)
		require.NoError(t, err)
		assert.Len(t, key, 32)
	})

	t.Run("generates unique keys", func(t *testing.T) {
		key1, err := generateRandomKey(32)
		require.NoError(t, err)

		key2, err := generateRandomKey(32)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "Generated keys should be unique")
	})

	t.Run("generates different lengths", func(t *testing.T) {
		key16, err := generateRandomKey(16)
		require.NoError(t, err)
		assert.Len(t, key16, 16)

		key64, err := generateRandomKey(64)
		require.NoError(t, err)
		assert.Len(t, key64, 64)
	})
}

func TestOnConfigurationChange_AutoGenerateEncryptionKey(t *testing.T) {
	t.Run("auto-generates encryption key when missing", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		config.Mattermost = mockAPI

		p := &Plugin{}
		p.SetAPI(mockAPI)

		// Configuration with valid secret but missing encryption key
		mockAPI.On("LoadPluginConfiguration", mock.AnythingOfType("*config.Configuration")).Run(func(args mock.Arguments) {
			cfg := args.Get(0).(*config.Configuration)
			cfg.Secret = "12345678901234567890123456789012" // 32 chars
			cfg.EncryptionKey = ""                          // Empty - should be auto-generated
		}).Return(nil)

		mockAPI.On("LogInfo", "Auto-generated missing Encryption Key.").Return()
		mockAPI.On("SavePluginConfig", mock.AnythingOfType("map[string]interface {}")).Return(nil)

		err := p.OnConfigurationChange()
		require.NoError(t, err)

		// Verify SavePluginConfig was called (meaning key was generated and saved)
		mockAPI.AssertCalled(t, "SavePluginConfig", mock.AnythingOfType("map[string]interface {}"))
	})

	t.Run("does not overwrite existing valid encryption key", func(t *testing.T) {
		mockAPI := &plugintest.API{}
		config.Mattermost = mockAPI

		p := &Plugin{}
		p.SetAPI(mockAPI)

		existingKey := "abcdefghijklmnopqrstuvwxyz123456" // 32 chars

		mockAPI.On("LoadPluginConfiguration", mock.AnythingOfType("*config.Configuration")).Run(func(args mock.Arguments) {
			cfg := args.Get(0).(*config.Configuration)
			cfg.Secret = "12345678901234567890123456789012"
			cfg.EncryptionKey = existingKey
		}).Return(nil)

		err := p.OnConfigurationChange()
		require.NoError(t, err)

		// SavePluginConfig should NOT be called since key is valid
		mockAPI.AssertNotCalled(t, "SavePluginConfig", mock.Anything)
	})
}
