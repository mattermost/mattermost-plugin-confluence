package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func setupMockAPI() *plugintest.API {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Roles: "system_user"}, nil)
	return mockAPI
}

func TestExecuteConfluenceDefault(t *testing.T) {
	mockAPI := setupMockAPI()

	cmdArgs := &model.CommandArgs{UserId: "U1", ChannelId: "C1"}
	resp := executeConfluenceDefault(nil, cmdArgs)

	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
	assert.True(t, strings.HasPrefix(resp.Text, invalidCommand))
	assert.Contains(t, resp.Text, commonHelpText)

	mockAPI.AssertExpectations(t)
}

func TestHandler_Handle_Default(t *testing.T) {
	mockAPI := setupMockAPI()

	p := &Plugin{}
	cmdArgs := &model.CommandArgs{UserId: "U1", ChannelId: "C1"}
	resp := ConfluenceCommandHandler.Handle(p, cmdArgs, "foo")

	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
	assert.Contains(t, resp.Text, invalidCommand)

	mockAPI.AssertExpectations(t)
}
