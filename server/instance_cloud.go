// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

// Atlassian Cloud 3LO endpoints.
// Docs: https://developer.atlassian.com/cloud/confluence/oauth-2-authorization-code-grants-3lo-for-apps/
const (
	cloudAuthURL                = "https://auth.atlassian.com/authorize"
	cloudTokenURL               = "https://auth.atlassian.com/oauth/token" //nolint:gosec
	cloudAudience               = "api.atlassian.com"
	cloudAccessibleResourcesURL = "https://api.atlassian.com/oauth/token/accessible-resources"
	cloudAPIBaseFmt             = "https://api.atlassian.com/ex/confluence/%s"
)

type AccessibleResource struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

func (p *Plugin) GetCloudOAuth2Config(isAdmin bool) (*oauth2.Config, error) {
	cfg := config.GetConfig()
	if cfg == nil {
		return nil, errors.New("error getting plugin configurations")
	}

	scopes := []string{
		"offline_access",
		"read:confluence-user",
		"read:confluence-content.summary",
		"read:confluence-content.all",
		"read:confluence-space.summary",
	}
	if isAdmin {
		scopes = append(scopes, "write:confluence-content")
	}

	return &oauth2.Config{
		ClientID:     cfg.ConfluenceOAuthClientID,
		ClientSecret: cfg.ConfluenceOAuthClientSecret,
		RedirectURL:  fmt.Sprintf("%s%s", util.GetPluginURL(), routeUserComplete),
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cloudAuthURL,
			TokenURL: cloudTokenURL,
		},
	}, nil
}

func (p *Plugin) GetCloudAuthCodeURL(state string, isAdmin bool) (string, error) {
	oconf, err := p.GetCloudOAuth2Config(isAdmin)
	if err != nil {
		return "", err
	}
	// audience is required by Atlassian; oauth2.Config doesn't know about it.
	return oconf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("audience", cloudAudience),
		oauth2.SetAuthURLParam("prompt", "consent"),
	), nil
}

// GetCloudAccessibleResources lists the Cloud sites the token can act on. The
// returned ID is the cloudId used to construct API URLs. Refresh on each token
// rotation since grants can change.
func (p *Plugin) GetCloudAccessibleResources(ctx context.Context, token *oauth2.Token) ([]AccessibleResource, error) {
	oconf, err := p.GetCloudOAuth2Config(false)
	if err != nil {
		return nil, err
	}

	httpClient := oconf.Client(ctx, token)
	httpClient.Timeout = 10 * time.Second

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cloudAccessibleResourcesURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build accessible-resources request")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call accessible-resources")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("accessible-resources returned %d", resp.StatusCode)
	}

	var resources []AccessibleResource
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, errors.Wrap(err, "failed to decode accessible-resources response")
	}
	return resources, nil
}

func (p *Plugin) GetCloudClient(instanceURL, cloudID string, connection *types.Connection) (Client, error) {
	oconf, err := p.GetCloudOAuth2Config(connection.IsAdmin)
	if err != nil {
		return nil, err
	}

	token, err := p.refreshAndStoreToken(connection, instanceURL, oconf)
	if err != nil {
		return nil, err
	}

	httpClient := oconf.Client(context.Background(), token)
	httpClient.Timeout = 10 * time.Second

	return newCloudClient(fmt.Sprintf(cloudAPIBaseFmt, cloudID), httpClient), nil
}

