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
	PathCurrentUser            = "/rest/api/user/current"
	PathCreateWebhook          = "/rest/api/webhooks"
	PathCommentData            = "/rest/api/content/%s?expand=body.view,container,space,history"
	PathPageData               = "/rest/api/content/%s?status=any&expand=body.view,container,space,history"
	PathSpaceData              = "/rest/api/space/%s?status=any"
	PathDeleteWebhook          = "/rest/api/webhooks/%s"
	PathGetUserGroupsForServer = "/rest/api/user/memberof?username=%s"
	PathAdminData              = "/rest/api/audit/retention"
)

const (
	Comment = "comment"
	Space   = "space"
	Page    = "page"
)

var webhookEvents = []string{"space_created", "space_removed", "space_updated",
	"page_created", "page_removed", "page_restored", "page_trashed",
	"page_updated", "comment_created", "comment_removed", "comment_updated"}

type WebhookConfiguration struct {
	Secret string `json:"secret"`
}

type CreateWebhookRequestBody struct {
	Name          string                `json:"name"`
	Events        []string              `json:"events"`
	URL           string                `json:"url"`
	Active        string                `json:"active"`
	Configuration *WebhookConfiguration `json:"configuration"`
}

type SpaceResponse struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Links Links  `json:"_links"`
}
type CommentContainer struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Links Links  `json:"_links"`
}

type Links struct {
	Self string `json:"webui"`
}

type View struct {
	Value string `json:"value"`
}
type Body struct {
	View View `json:"view"`
}

type CreatedBy struct {
	Username string `json:"username"`
}

type History struct {
	CreatedBy CreatedBy `json:"createdBy"`
}

type CommentResponse struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Space     SpaceResponse    `json:"space"`
	Container CommentContainer `json:"container"`
	Body      Body             `json:"body"`
	Links     Links            `json:"_links"`
	History   History          `json:"history"`
}

type PageResponse struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Space   SpaceResponse `json:"space"`
	Body    Body          `json:"body"`
	Links   Links         `json:"_links"`
	History History       `json:"history"`
}

type ConfluenceServerEvent struct {
	Comment *CommentResponse
	Page    *PageResponse
	Space   *SpaceResponse
	BaseURL string
}

type confluenceServerClient struct {
	URL        string
	HTTPClient *http.Client
}

type ConfluenceServerUser struct {
	UserKey     string `json:"userKey"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type AdminData struct {
	Number int    `json:"number"`
	Units  string `json:"units"`
}

type WebhookResponse struct {
	ID int `json:"id"`
}

func newServerClient(url string, httpClient *http.Client) Client {
	return &confluenceServerClient{
		URL:        url,
		HTTPClient: httpClient,
	}
}

func (csc *confluenceServerClient) CreateWebhook(subscription serializer.Subscription, redirectURL, secret string) (*WebhookResponse, error) {
	requestBody := &CreateWebhookRequestBody{
		Name:   subscription.GetAlias(),
		Events: webhookEvents,
		URL:    redirectURL,
		Active: "true",
		Configuration: &WebhookConfiguration{
			Secret: secret,
		},
	}
	webhookResponse := &WebhookResponse{}
	_, err := service.CallJSONWithURL(csc.URL, PathCreateWebhook, http.MethodPost, requestBody, webhookResponse, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence CreateWebhook")
	}

	return webhookResponse, nil
}

func (csc *confluenceServerClient) DeleteWebhook(webhookID string) error {
	_, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf(PathDeleteWebhook, webhookID), http.MethodDelete, nil, nil, csc.HTTPClient)
	if err != nil {
		return errors.Wrap(err, "confluence DeleteWebhook")
	}

	return nil
}

func (csc *confluenceServerClient) GetSelf() (*ConfluenceUser, error) {
	confluenceServerUser := &ConfluenceServerUser{}
	_, err := service.CallJSONWithURL(csc.URL, PathCurrentUser, http.MethodGet, nil, confluenceServerUser, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetSelf")
	}

	confluenceUser := &ConfluenceUser{
		AccountID:   confluenceServerUser.UserKey,
		Name:        confluenceServerUser.Username,
		DisplayName: confluenceServerUser.DisplayName,
	}

	return confluenceUser, nil
}

func (csc *confluenceServerClient) GetEventData(webhookPayload *serializer.ConfluenceServerWebhookPayload) (*ConfluenceServerEvent, error) {
	var confluenceServerEvent ConfluenceServerEvent
	var err error
	if strings.Contains(webhookPayload.Event, Comment) {
		confluenceServerEvent.Comment, err = csc.GetCommentData(webhookPayload)
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}
	}
	if strings.Contains(webhookPayload.Event, Page) {
		confluenceServerEvent.Page, err = csc.GetPageData(int(webhookPayload.Page.ID))
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}
	}
	if strings.Contains(webhookPayload.Event, Space) {
		confluenceServerEvent.Space, err = csc.GetSpaceData(webhookPayload.Space.SpaceKey)
		if err != nil {
			return nil, errors.Wrap(err, "confluence GetEventData")
		}
	}

	return &confluenceServerEvent, nil
}

func (csc *confluenceServerClient) GetCommentData(webhookPayload *serializer.ConfluenceServerWebhookPayload) (*CommentResponse, error) {
	commentResponse := &CommentResponse{}
	_, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf(PathCommentData, strconv.FormatInt(webhookPayload.Comment.ID, 10)), http.MethodGet, nil, commentResponse, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetCommentData")
	}

	commentResponse.Body.View.Value = utils.GetBodyForExcerpt(commentResponse.Body.View.Value)

	return commentResponse, nil
}

func (csc *confluenceServerClient) GetPageData(pageID int) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	_, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf(PathPageData, strconv.Itoa(pageID)), http.MethodGet, nil, pageResponse, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetPageData")
	}

	pageResponse.Body.View.Value = utils.GetBodyForExcerpt(pageResponse.Body.View.Value)

	return pageResponse, nil
}

func (csc *confluenceServerClient) GetSpaceData(spaceKey string) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	_, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf(PathSpaceData, spaceKey), http.MethodGet, nil, spaceResponse, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetSpaceData")
	}

	return spaceResponse, nil
}

func (csc *confluenceServerClient) CheckConfluenceAdmin() (*AdminData, error) {
	adminData := &AdminData{}
	_, err := service.CallJSONWithURL(csc.URL, PathAdminData, http.MethodGet, nil, adminData, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence CheckConfluenceAdmin")
	}

	return adminData, nil
}

func (csc *confluenceServerClient) GetUserGroups(connection *Connection) ([]*UserGroup, error) {
	userGroups := UserGroups{}

	_, err := service.CallJSONWithURL(csc.URL, PathGetUserGroupsForServer, http.MethodGet, nil, &userGroups, csc.HTTPClient)
	if err != nil {
		return nil, errors.Wrap(err, "confluence GetUserGroups")
	}

	return userGroups.Groups, nil
}
