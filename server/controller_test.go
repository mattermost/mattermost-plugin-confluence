package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const testWebhookKey = "test-webhook-secret"

func allowLogs(mockAPI *plugintest.API) {
	for _, level := range []string{"LogDebug", "LogInfo", "LogWarn", "LogError"} {
		for n := 1; n <= 15; n++ {
			args := make([]interface{}, n)
			for i := range args {
				args[i] = mock.Anything
			}
			mockAPI.On(level, args...).Maybe().Return()
		}
	}
}

func setupWebhookRouter(t *testing.T, serverVersionGreaterThan9 bool) *mux.Router {
	t.Helper()

	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	allowLogs(mockAPI)
	mockAPI.On("GetBundlePath").Return(t.TempDir(), nil)
	mockAPI.On("KVGet", mock.Anything).Return(nil, nil).Maybe()

	config.SetConfig(&config.Configuration{
		Secret:                    testWebhookKey,
		ConfluenceURL:             "https://confluence.example.com",
		ServerVersionGreaterthan9: serverVersionGreaterThan9,
	})

	p := &Plugin{}
	p.SetAPI(mockAPI)
	p.client = pluginapi.NewClient(mockAPI, nil)

	return p.InitAPI()
}

func postServerWebhook(router http.Handler, secret, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/webhook?secret="+secret, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestConfluenceServerWebhook(t *testing.T) {
	for name, tc := range map[string]struct {
		serverVersionGreaterThan9 bool
		secret                    string
		body                      string
		expectedStatus            int
	}{
		"wrong secret": {
			serverVersionGreaterThan9: true,
			secret:                    "wrong",
			body:                      `{"test":true}`,
			expectedStatus:            http.StatusForbidden,
		},
		"test connection": {
			serverVersionGreaterThan9: true,
			secret:                    testWebhookKey,
			body:                      `{"test":true}`,
			expectedStatus:            http.StatusOK,
		},
		"null payload": {
			serverVersionGreaterThan9: true,
			secret:                    testWebhookKey,
			body:                      `null`,
			expectedStatus:            http.StatusBadRequest,
		},
		"padded null payload": {
			serverVersionGreaterThan9: true,
			secret:                    testWebhookKey,
			body:                      " \n\tnull \n",
			expectedStatus:            http.StatusBadRequest,
		},
		"valid payload from unconnected user": {
			serverVersionGreaterThan9: true,
			secret:                    testWebhookKey,
			body:                      `{"event":"unsupported_event","userKey":"unknown"}`,
			expectedStatus:            http.StatusOK,
		},
		"legacy payload": {
			serverVersionGreaterThan9: false,
			secret:                    testWebhookKey,
			body:                      `{}`,
			expectedStatus:            http.StatusOK,
		},
		"legacy null payload": {
			serverVersionGreaterThan9: false,
			secret:                    testWebhookKey,
			body:                      `null`,
			expectedStatus:            http.StatusOK,
		},
	} {
		t.Run(name, func(t *testing.T) {
			router := setupWebhookRouter(t, tc.serverVersionGreaterThan9)

			w := postServerWebhook(router, tc.secret, tc.body)

			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestWrapHandlerRecoversFromPanic(t *testing.T) {
	mockAPI := &plugintest.API{}
	allowLogs(mockAPI)

	p := &Plugin{}
	p.client = pluginapi.NewClient(mockAPI, nil)

	handler := p.wrapHandler(func(http.ResponseWriter, *http.Request, *Plugin) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	assert.NotPanics(t, func() {
		handler(w, httptest.NewRequest(http.MethodPost, "/api/v1/server/webhook?secret="+testWebhookKey, nil))
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	for _, call := range mockAPI.Calls {
		for _, arg := range call.Arguments {
			if s, ok := arg.(string); ok {
				assert.NotContains(t, s, testWebhookKey)
			}
		}
	}
}
