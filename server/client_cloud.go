// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

// confluenceCloudClient calls the Atlassian Cloud REST API.
// APIBase is https://api.atlassian.com/ex/confluence/{cloudId}; callers append
// paths such as "/wiki/api/v2/...".
type confluenceCloudClient struct {
	APIBase    string
	HTTPClient *http.Client
}

func newCloudClient(apiBase string, httpClient *http.Client) Client {
	return &confluenceCloudClient{APIBase: apiBase, HTTPClient: httpClient}
}

type cloudCurrentUser struct {
	AccountID   string `json:"accountId"`
	PublicName  string `json:"publicName"`
	DisplayName string `json:"displayName"`
}

// GetSelf returns the authenticated user via the Atlassian platform "me"
// endpoint. The Confluence v2 user endpoints require a known accountId; "me"
// is the canonical way to discover that with just an OAuth token.
func (c *confluenceCloudClient) GetSelf() (*types.ConfluenceUser, error) {
	req, err := http.NewRequest(http.MethodGet, c.APIBase+"/wiki/rest/api/user/current", nil)
	if err != nil {
		return nil, errors.Wrap(err, "build cloud GetSelf request")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "cloud GetSelf request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("cloud GetSelf returned %d", resp.StatusCode)
	}

	var u cloudCurrentUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, errors.Wrap(err, "decode cloud GetSelf response")
	}

	name := u.PublicName
	if name == "" {
		name = u.DisplayName
	}
	return &types.ConfluenceUser{
		AccountID:   u.AccountID,
		Name:        name,
		DisplayName: u.DisplayName,
	}, nil
}

// GetSpaceData / GetPageData / GetSpaceKeyFromSpaceID are only invoked by the
// ServerVersionGreaterthan9 subscription-validation path (see
// validateUserConfluenceAccess in user.go) which does not fire for Cloud
// installs. They stay stubbed until Cloud subscriptions need server-side
// validation against Confluence Cloud REST v2.

func (c *confluenceCloudClient) GetSpaceData(string) (*SpaceResponse, error) {
	return nil, errors.New("GetSpaceData is not implemented for Confluence Cloud")
}

func (c *confluenceCloudClient) GetPageData(int) (*PageResponse, error) {
	return nil, errors.New("GetPageData is not implemented for Confluence Cloud")
}

func (c *confluenceCloudClient) GetSpaceKeyFromSpaceID(int64) (string, error) {
	return "", errors.New("GetSpaceKeyFromSpaceID is not implemented for Confluence Cloud")
}
