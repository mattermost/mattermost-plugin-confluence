package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func TestHandleConfluenceConfig(t *testing.T) {
	validUserID, validChannelID := getValidUserAndChannelID()
	tests := map[string]struct {
		method         string
		statusCode     int
		body           string
		userID         string
		channelID      string
		patchFuncCalls func()
	}{
		"success": {
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			body: `{
				"type": "dialog_submission",
				"callback_id": "callbackID",
				"state": "", 
				"submission": {
					"Client ID": "mock-ClientID",
					"Client Secret": "mock-ClientSecret",
					"Server URL": "https://test.com"
				},
				"canceled": false
			}`,
			userID:    validUserID,
			channelID: validChannelID,
			patchFuncCalls: func() {
				monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetConfigKeyList", func(_ *Plugin) ([]string, error) {
					return []string{"https://test.com"}, nil
				})
			},
		},
		"wrong API method": {
			method:     http.MethodGet,
			statusCode: http.StatusMethodNotAllowed,
			userID:     validUserID,
			channelID:  validChannelID,
		},
		"invalid body": {
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       `{`,
			userID:     validUserID,
			channelID:  validChannelID,
			patchFuncCalls: func() {
				monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetConfigKeyList", func(_ *Plugin) ([]string, error) {
					return []string{"https://test.com"}, nil
				})
			},
		},
		"invalid userID or channelID": {
			method:     http.MethodPost,
			statusCode: http.StatusBadRequest,
			body:       `{`,
			userID:     "mock-userID",
			channelID:  "mockChannelID",
			patchFuncCalls: func() {
				monkey.PatchInstanceMethod(reflect.TypeOf(&Plugin{}), "GetConfigKeyList", func(_ *Plugin) ([]string, error) {
					return []string{"https://test.com"}, nil
				})
			},
		},
	}
	mockAPI, p := getMockAPIAndPlugin()

	p.userStore = getMockUserStoreKV()
	p.instanceStore = p.getMockInstanceStoreKV(1)
	p.otsStore = p.getMockOTSStoreKV()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()

			mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.AnythingOfType("*model.Post")).Once().Return(&model.Post{})
			mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(getMockUser(true), nil)

			if tc.patchFuncCalls != nil {
				tc.patchFuncCalls()
			}

			request := httptest.NewRequest(tc.method, fmt.Sprintf("/api/v1/config/%s/%s", tc.channelID, tc.userID), bytes.NewBufferString(tc.body))
			request.Header.Set(config.HeaderMattermostUserID, "test-user")
			w := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, w, request)
			assert.Equal(t, tc.statusCode, w.Result().StatusCode)
		})
	}
}
