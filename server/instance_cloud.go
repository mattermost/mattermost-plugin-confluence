package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type cloudInstance struct {
	CloudID string

	*InstanceCommon
}

func (p *Plugin) installCloudInstance(rawURL string) (string, *cloudInstance, error) {
	confluenceURL, err := service.NormalizeConfluenceURL(rawURL)
	if err != nil {
		return "", nil, err
	}
	if !strings.HasPrefix(confluenceURL, "https://") {
		return "", nil, errors.New("a secure HTTPS URL is required")
	}

	instance := &cloudInstance{
		InstanceCommon: newInstanceCommon(p, CloudInstanceType, types.ID(confluenceURL)),
	}

	if err = p.InstallInstance(instance, false); err != nil {
		return "", nil, err
	}

	return confluenceURL, instance, err
}

func (ci *cloudInstance) GetOAuth2Config(isAdmin bool) (*oauth2.Config, error) {
	config, err := ci.Plugin.instanceStore.LoadInstanceConfig(ci.GetURL())
	if err != nil {
		return nil, fmt.Errorf(configNotFoundError, ci.InstanceID, ci.InstanceID)
	}

	return &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  fmt.Sprintf("%s%s", ci.GetPluginURL(), instancePath(routeUserComplete, ci.InstanceID)),
		Scopes: []string{
			"read:confluence-user",
			"read:confluence-content.all",
			"read:content-details:confluence",
			"write:confluence-content",
			"offline_access",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://auth.atlassian.com/authorize?audience=api.atlassian.com",
			TokenURL: "https://auth.atlassian.com/oauth/token",
		},
	}, nil
}

func (ci *cloudInstance) GetURL() string {
	return ci.InstanceID.String()
}

func (ci *cloudInstance) GetManageAppsURL() string {
	return fmt.Sprintf("%s/plugins/servlet/upm", ci.GetURL())
}

func (ci *cloudInstance) GetManageWebhooksURL() string {
	return cloudManageWebhooksURL(ci.GetURL())
}

func cloudManageWebhooksURL(confluenceURL string) string {
	return fmt.Sprintf("%s/plugins/servlet/webhooks", confluenceURL)
}

func (ci *cloudInstance) GetClient(connection *Connection) (Client, error) {
	oconf, err := ci.GetOAuth2Config(connection.IsAdmin)
	if err != nil {
		return nil, err
	}

	token, err := ci.Plugin.refreshAndStoreToken(connection, ci.InstanceID, oconf)
	if err != nil {
		return nil, err
	}

	httpClient := oconf.Client(context.Background(), token)

	baseURL := "https://api.atlassian.com"
	if ci.CloudID != "" {
		baseURL = fmt.Sprintf("%s/ex/confluence/%s/wiki", baseURL, ci.CloudID)
	}

	return newCloudClient(baseURL, ci.InstanceID.String(), httpClient), nil
}
