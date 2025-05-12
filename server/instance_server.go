package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

func (p *Plugin) GetServerOAuth2Config(instanceURL string, isAdmin bool) (*oauth2.Config, error) {
	config := config.GetConfig()
	if config == nil {
		return nil, errors.New("error getting plugin configurations")
	}

	var scopes []string
	if isAdmin {
		scopes = []string{
			"ADMIN",
		}
	} else {
		scopes = []string{
			"READ",
			"WRITE",
		}
	}
	return &oauth2.Config{
		ClientID:     config.ConfluenceOAuthClientID,
		ClientSecret: config.ConfluenceOAuthClientSecret,
		RedirectURL:  fmt.Sprintf("%s%s", util.GetPluginURL(), routeUserComplete),
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/rest/oauth2/latest/authorize", instanceURL),
			TokenURL: fmt.Sprintf("%s/rest/oauth2/latest/token", instanceURL),
		},
	}, nil
}

func (p *Plugin) GetServerClient(instanceID string, connection *types.Connection) (Client, error) {
	oconf, err := p.GetServerOAuth2Config(instanceID, connection.IsAdmin)
	if err != nil {
		return nil, err
	}

	token, err := p.refreshAndStoreToken(connection, instanceID, oconf)
	if err != nil {
		return nil, err
	}
	httpClient := oconf.Client(context.Background(), token)
	httpClient.Timeout = 10 * time.Second

	return newServerClient(instanceID, httpClient), nil
}

func (p *Plugin) GetRedirectURL() string {
	return fmt.Sprintf("%s%s", util.GetPluginURL(), routeUserComplete)
}
