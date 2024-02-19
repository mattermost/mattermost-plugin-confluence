package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type serverInstance struct {
	*InstanceCommon
}

func (p *Plugin) installServerInstance(rawURL string) (string, *serverInstance, error) {
	confluenceURL, err := service.CheckConfluenceURL(p.GetSiteURL(), rawURL, false)
	if err != nil {
		return "", nil, err
	}
	if utils.IsConfluenceCloudURL(confluenceURL) {
		return "", nil, errors.Errorf("`%s` is not a Confluence server URL, it refers to Confluence Cloud", confluenceURL)
	}

	instance := &serverInstance{
		InstanceCommon: newInstanceCommon(p, ServerInstanceType, types.ID(confluenceURL)),
	}

	if err = p.InstallInstance(instance, false); err != nil {
		return "", nil, err
	}

	return confluenceURL, instance, err
}

func (si *serverInstance) GetURL() string {
	return si.InstanceID.String()
}

func (si *serverInstance) GetManageAppsURL() string {
	return fmt.Sprintf("%s/plugins/servlet/applinks/listApplicationLinks", si.GetURL())
}

func (si *serverInstance) GetManageWebhooksURL() string {
	return fmt.Sprintf("%s/plugins/servlet/webhooks", si.GetURL())
}

func (si *serverInstance) GetOAuth2Config(isAdmin bool) (*oauth2.Config, error) {
	config, err := si.Plugin.instanceStore.LoadInstanceConfig(si.GetURL())
	if err != nil {
		return nil, fmt.Errorf(configNotFoundError, si.InstanceID, si.InstanceID)
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
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  fmt.Sprintf("%s%s", si.GetPluginURL(), instancePath(routeUserComplete, si.InstanceID)),
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/rest/oauth2/latest/authorize", si.GetURL()),
			TokenURL: fmt.Sprintf("%s/rest/oauth2/latest/token", si.GetURL()),
		},
	}, nil
}

func (si *serverInstance) GetClient(connection *Connection) (Client, error) {
	oconf, err := si.GetOAuth2Config(connection.IsAdmin)
	if err != nil {
		return nil, err
	}

	token, err := si.Plugin.refreshAndStoreToken(connection, si.InstanceID, oconf)
	if err != nil {
		return nil, err
	}
	httpClient := oconf.Client(context.Background(), token)

	return newServerClient(si.GetURL(), httpClient), nil
}
