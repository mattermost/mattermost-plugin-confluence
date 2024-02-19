package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const (
	validUserID    = "iu73atknztnctef8b8ey9gm6zc"
	validChannelID = "tgniw3kmrjd93qns11cboditme"
)

func baseMock() *plugintest.API {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI

	return mockAPI
}

func mockAnythingOfTypeBatch(argType string, numCalls int) []interface{} {
	args := make([]interface{}, numCalls)

	for i := 0; i < numCalls; i++ {
		args[i] = mock.AnythingOfType(argType)
	}

	return args
}

func getMockAPIAndPlugin() (*plugintest.API, *Plugin) {
	mockAPI := baseMock()
	mockAPI.On("LogError", mockAnythingOfTypeBatch("string", 13)...)
	mockAPI.On("LogDebug", mockAnythingOfTypeBatch("string", 11)...)
	mockAPI.On("GetBundlePath").Return("/test/testBundlePath", nil)

	p := Plugin{}
	p.SetAPI(mockAPI)

	return mockAPI, &p
}

func getValidUserAndChannelID() (string, string) {
	return validUserID, validChannelID
}

func getMockUser(isAdmin bool) *model.User {
	if isAdmin {
		return &model.User{Id: "123", Roles: model.SystemAdminRoleId}
	}

	return &model.User{Id: "123", Roles: model.SystemUserRoleId}
}
