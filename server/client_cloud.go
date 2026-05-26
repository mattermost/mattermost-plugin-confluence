// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/service"
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

// Returned on 401/403 from v2 endpoints; signals the actor needs to reconnect
// to grant the granular page/comment scopes.
var ErrCloudInsufficientScope = errors.New("cloud token lacks required scopes; user must reconnect")

type cloudADFBodyResponse struct {
	Body struct {
		AtlasDocFormat struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"atlas_doc_format"`
	} `json:"body"`
}

func (c *confluenceCloudClient) MentionAccountIDsInPage(pageID string) ([]string, error) {
	body, err := c.fetchADF("/wiki/api/v2/pages/" + pageID)
	if err != nil || len(body) == 0 {
		return nil, err
	}
	return service.ExtractMentionAccountIDsFromADF(body)
}

func (c *confluenceCloudClient) MentionAccountIDsInComment(commentID, location string) ([]string, error) {
	path := "/wiki/api/v2/footer-comments/" + commentID
	if location == "inline" {
		path = "/wiki/api/v2/inline-comments/" + commentID
	}
	body, err := c.fetchADF(path)
	if err != nil || len(body) == 0 {
		return nil, err
	}
	return service.ExtractMentionAccountIDsFromADF(body)
}

func (c *confluenceCloudClient) fetchADF(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.APIBase+path+"?body-format=atlas_doc_format", nil)
	if err != nil {
		return nil, errors.Wrap(err, "build cloud ADF request")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "cloud ADF request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrCloudInsufficientScope
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("cloud ADF fetch returned %d", resp.StatusCode)
	}

	var wrap cloudADFBodyResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil {
		return nil, errors.Wrap(err, "decode cloud ADF response")
	}
	return []byte(wrap.Body.AtlasDocFormat.Value), nil
}
