package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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
