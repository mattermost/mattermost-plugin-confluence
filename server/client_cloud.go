// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/util"
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

// v1 REST paths: the plugin's classic OAuth scopes do not authorize the v2 API,
// which would need a re-consent from every existing install.
const cloudAPIPrefix = "/wiki"

func (c *confluenceCloudClient) getJSON(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.APIBase+cloudAPIPrefix+path, nil)
	if err != nil {
		return errors.Wrapf(err, "build cloud request for %s", path)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "cloud request for %s failed", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("cloud request for %s returned %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return errors.Wrapf(err, "decode cloud response for %s", path)
	}

	return nil
}

// GetSelf returns the authenticated user via the Atlassian platform "me"
// endpoint. The Confluence v2 user endpoints require a known accountId; "me"
// is the canonical way to discover that with just an OAuth token.
func (c *confluenceCloudClient) GetSelf() (*types.ConfluenceUser, error) {
	var u cloudCurrentUser
	if err := c.getJSON(PathCurrentUser, &u); err != nil {
		return nil, errors.Wrap(err, "Confluence GetSelf. Error getting the current user")
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

func (c *confluenceCloudClient) GetSpaceData(spaceKey string) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	if err := c.getJSON(fmt.Sprintf("%s%s?status=any", PathSpaceData, url.PathEscape(spaceKey)), spaceResponse); err != nil {
		return nil, err
	}

	return spaceResponse, nil
}

func (c *confluenceCloudClient) GetPageData(pageID int) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	if err := c.getJSON(fmt.Sprintf("%s%d?status=any&expand=body.view,space,history", PathContentData, pageID), pageResponse); err != nil {
		return nil, err
	}

	pageResponse.Body.View.Value = util.GetBodyForExcerpt(pageResponse.Body.View.Value)

	return pageResponse, nil
}

// Cloud events arrive through the Forge bridge already carrying the space key.
func (c *confluenceCloudClient) GetSpaceKeyFromSpaceID(int64) (string, error) {
	return "", errors.New("GetSpaceKeyFromSpaceID is not implemented for Confluence Cloud")
}

func (c *confluenceCloudClient) MentionAccountIDsInPage(string) ([]string, error) {
	return nil, errors.New("not supported on Cloud: mentions are parsed from the Forge event body")
}

func (c *confluenceCloudClient) MentionAccountIDsInComment(string, string) ([]string, error) {
	return nil, errors.New("not supported on Cloud: mentions are parsed from the Forge event body")
}
