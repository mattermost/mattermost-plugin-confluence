package main

import (
	"encoding/json"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

func TestStartCompletionWizard(t *testing.T) {
	for name, tc := range map[string]struct {
		isCloud                 bool
		roles                   string
		getUserErr              *model.AppError
		expectSetupInstructions bool
	}{
		"server mode, system admin": {
			roles:                   model.SystemUserRoleId + " " + model.SystemAdminRoleId,
			expectSetupInstructions: true,
		},
		"server mode, regular user": {
			roles: model.SystemUserRoleId,
		},
		"server mode, user lookup fails": {
			getUserErr: model.NewAppError("GetUser", "not_found", nil, "", 404),
		},
		"cloud mode, system admin": {
			isCloud: true,
			roles:   model.SystemUserRoleId + " " + model.SystemAdminRoleId,
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			config.Mattermost = mockAPI
			config.SetConfig(&config.Configuration{
				ConfluenceURL: "https://confluence.example.com",
				IsCloud:       tc.isCloud,
			})

			siteURL := "https://mattermost.example.com"
			mockAPI.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: &siteURL}})
			mockAPI.On("LogDebug", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

			var user *model.User
			if tc.getUserErr == nil {
				user = &model.User{Id: "user-id", Roles: tc.roles}
			}
			mockAPI.On("GetUser", "user-id").Return(user, tc.getUserErr).Maybe()

			mockAPI.On("KVSetWithOptions", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
			mockAPI.On("GetDirectChannel", "bot-id", "user-id").Return(&model.Channel{Id: "dm-channel"}, nil)

			var rendered string
			mockAPI.On("CreatePost", mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
				b, err := json.Marshal(args.Get(0))
				require.NoError(t, err)
				rendered = string(b)
			}).Return(&model.Post{Id: "post-id"}, nil)

			p := &Plugin{BotUserID: "bot-id", Router: mux.NewRouter()}
			p.SetAPI(mockAPI)
			p.client = pluginapi.NewClient(mockAPI, nil)

			fm, err := p.NewFlowManager()
			require.NoError(t, err)

			require.NoError(t, fm.StartCompletionWizard("user-id"))
			require.Contains(t, rendered, "connected")

			if tc.expectSetupInstructions {
				assert.Contains(t, rendered, "/server/webhook")
			} else {
				assert.NotContains(t, rendered, "/server/webhook")
			}
		})
	}
}
