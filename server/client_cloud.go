// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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

// v1 REST paths: the plugin's classic OAuth scopes do not authorize the v2 API,
// which would need a re-consent from every existing install.
const cloudAPIPrefix = "/wiki"

// getJSON GETs path relative to the Cloud API base and decodes the JSON
// response body into out. A non-200 response yields an *APIError so callers can
// act on the upstream status.
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
		return &APIError{StatusCode: resp.StatusCode, Path: path}
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

// Cloud answers 410 for GET /space/{key} and /content/{id} over OAuth, and their v2 replacements need granular
// scopes that existing installs never granted. Only access checks call these, so they read the space settings
// and the page restrictions instead: both need view permission and accept the classic scopes.
func (c *confluenceCloudClient) GetSpaceData(spaceKey string) (*SpaceResponse, error) {
	var settings struct {
		SpaceKey string `json:"spaceKey"`
	}
	if err := c.getJSON(fmt.Sprintf("%s%s/settings", PathSpaceData, url.PathEscape(spaceKey)), &settings); err != nil {
		return nil, err
	}

	if settings.SpaceKey == "" {
		settings.SpaceKey = spaceKey
	}
	return &SpaceResponse{Key: settings.SpaceKey}, nil
}

func (c *confluenceCloudClient) GetPageData(pageID int) (*PageResponse, error) {
	var restrictions json.RawMessage
	if err := c.getJSON(fmt.Sprintf("%s%d/restriction", PathContentData, pageID), &restrictions); err != nil {
		return nil, err
	}

	return &PageResponse{ID: strconv.Itoa(pageID)}, nil
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
