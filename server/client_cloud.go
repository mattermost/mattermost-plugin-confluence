package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const (
	PathAccessibleResources   = "/oauth/token/accessible-resources"
	PathGetUserGroupsForCloud = "rest/api/user/memberof?accountId=%s"
)

type confluenceCloudClient struct {
	URL        string
	InstanceID string
	HTTPClient *http.Client
}

type ConfluenceCloudEvent struct {
	Comment *CommentResponse
	Page    *PageResponse
	Space   *SpaceResponse
}

type AccessibleResources struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Scopes    []string `json:"scopes"`
	AvatarURL string   `json:"avatarUrl"`
}

type ConfluenceCloudUser struct {
	AccountID   string `json:"accountId"`
	PublicName  string `json:"publicName"`
	DisplayName string `json:"displayName"`
}

func newCloudClient(url, instanceID string, httpClient *http.Client) Client {
	return &confluenceCloudClient{
		URL:        url,
		InstanceID: instanceID,
		HTTPClient: httpClient,
	}
}

func (ccc *confluenceCloudClient) GetSelf() (*ConfluenceUser, error) {
	confluenceCloudUser := &ConfluenceCloudUser{}
	_, err := service.CallJSONWithURL(ccc.URL, PathCurrentUser, http.MethodGet, nil, confluenceCloudUser, ccc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetSelf")
	}

	confluenceUser := &ConfluenceUser{
		AccountID:   confluenceCloudUser.AccountID,
		Name:        confluenceCloudUser.PublicName,
		DisplayName: confluenceCloudUser.DisplayName,
	}
	return confluenceUser, nil
}

func (ccc *confluenceCloudClient) GetCloudID() (string, error) {
	accessibleResources := []*AccessibleResources{}
	_, err := service.CallJSONWithURL(ccc.URL, PathAccessibleResources, http.MethodGet, nil, &accessibleResources, ccc.HTTPClient)
	if err != nil {
		return "", errors.Wrap(err, "confluence GetCloudID")
	}

	for _, accessibleResource := range accessibleResources {
		if accessibleResource.URL == ccc.InstanceID {
			return accessibleResource.ID, nil
		}
	}

	return "", fmt.Errorf("cloudID not found for cloud instance: %s", ccc.InstanceID)
}

func (ccc *confluenceCloudClient) GetEventData(confluenceCloudEvent *serializer.ConfluenceCloudEvent, eventType string) (*ConfluenceCloudEvent, error) {
	var err error
	var confluenceCloudEventResponse ConfluenceCloudEvent
	if strings.Contains(eventType, Comment) {
		confluenceCloudEventResponse.Comment, err = ccc.GetCommentData(confluenceCloudEvent)
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}

		confluenceCloudEvent.Comment.Body.View.Value = confluenceCloudEventResponse.Comment.Body.View.Value
		confluenceCloudEvent.Comment.UserName = confluenceCloudEventResponse.Comment.History.CreatedBy.Username
	}
	if strings.Contains(eventType, Page) {
		confluenceCloudEventResponse.Page, err = ccc.GetPageData(confluenceCloudEvent.Page.ID)
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}

		confluenceCloudEvent.Page.Body.View.Value = confluenceCloudEventResponse.Page.Body.View.Value
		confluenceCloudEvent.Page.UserName = confluenceCloudEventResponse.Page.History.CreatedBy.Username
	}
	if strings.Contains(eventType, Space) {
		confluenceCloudEventResponse.Space, err = ccc.GetSpaceData(confluenceCloudEvent.Space.SpaceKey)
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}
	}

	return &confluenceCloudEventResponse, nil
}

func (ccc *confluenceCloudClient) GetCommentData(confluenceCloudEvent *serializer.ConfluenceCloudEvent) (*CommentResponse, error) {
	commentResponse := &CommentResponse{}
	_, err := service.CallJSONWithURL(ccc.URL, fmt.Sprintf(PathCommentData, strconv.Itoa(confluenceCloudEvent.Comment.ID)), http.MethodGet, nil, commentResponse, ccc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetCommentData")
	}

	commentResponse.Body.View.Value = utils.GetBodyForExcerpt(commentResponse.Body.View.Value)

	return commentResponse, nil
}

func (ccc *confluenceCloudClient) GetPageData(pageID int) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	_, err := service.CallJSONWithURL(ccc.URL, fmt.Sprintf(PathPageData, strconv.Itoa(pageID)), http.MethodGet, nil, pageResponse, ccc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetPageData")
	}

	pageResponse.Body.View.Value = utils.GetBodyForExcerpt(pageResponse.Body.View.Value)

	return pageResponse, nil
}

func (ccc *confluenceCloudClient) GetSpaceData(spaceKey string) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	_, err := service.CallJSONWithURL(ccc.URL, fmt.Sprintf(PathSpaceData, spaceKey), http.MethodGet, nil, spaceResponse, ccc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetSpaceData")
	}

	return spaceResponse, nil
}

func (ccc *confluenceCloudClient) GetUserGroups(connection *Connection) ([]*UserGroup, error) {
	userGroups := UserGroups{}
	_, err := service.CallJSONWithURL(ccc.URL, fmt.Sprintf(PathGetUserGroupsForCloud, connection.AccountID), http.MethodGet, nil, &userGroups, ccc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetUserGroups")
	}

	return userGroups.Groups, nil
}
