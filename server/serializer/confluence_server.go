package serializer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const (
	ConfluenceContentTypePage     = "page"
	ConfluenceContentTypeBlogPost = "blogpost"
	ConfluenceContentTypeComment  = "comment"

	ConfluencePageCreatedMessage            = "%s published a new page in %s."
	ConfluencePageCreatedWithoutBodyMessage = "%s published a new page %s in %s."
	ConfluencePageUpdatedMessage            = "%s updated %s in %s."
	ConfluencePageTrashedMessage            = "%s trashed %s in %s."
	ConfluencePageRestoredMessage           = "%s restored %s in %s."
	ConfluencePageRemovedMessage            = "%s removed **%s** in."
	ConfluenceCommentCreatedMessage         = "%s commented on %s in %s."
	ConfluenceEmptyCommentCreatedMessage    = "%s [commented](%s) on %s in %s."
	ConfluenceCommentReplyCreatedMessage    = "%s replied to a comment on %s in %s."
	ConfluenceCommentUpdatedMessage         = "%s updated a comment on %s in %s."
	ConfluenceEmptyCommentUpdatedMessage    = "%s updated a [comment](%s) on %s in %s."
	ConfluenceCommentRemovedMessage         = "%s removed a comment on %s in %s."
	ConfluenceSpaceUpdatedMessage           = "A space titled [%s](%s) was updated."
)

type ConfluenceServerUser struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	URL      string `json:"url"`
	Username string `json:"username"`
}

type ConfluenceServerSpace struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	Status      string `json:"status"`
}

type ConfluenceServerPageAncestor struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ConfluenceServerPage struct {
	IsHomePage    bool                           `json:"is_home_page"`
	CreatedAt     int64                          `json:"created_at"`
	Title         string                         `json:"title"`
	Content       string                         `json:"content"`
	HTMLContent   string                         `json:"html_content"`
	IsDeleted     bool                           `json:"is_deleted"`
	ContentType   string                         `json:"content_type"`
	UpdatedAt     int64                          `json:"updated_at"`
	IsDraft       bool                           `json:"is_draft"`
	ID            string                         `json:"id"`
	Ancestors     []ConfluenceServerPageAncestor `json:"ancestors"`
	IsRootLevel   bool                           `json:"is_root_level"`
	ContentID     int                            `json:"content_id"`
	IsIndexable   bool                           `json:"is_indexable"`
	Version       int                            `json:"version"`
	CreatedBy     ConfluenceServerUser           `json:"created_by"`
	URL           string                         `json:"url"`
	Labels        []string                       `json:"labels"`
	TinyURL       string                         `json:"tiny_url"`
	UpdatedBy     ConfluenceServerUser           `json:"updated_by"`
	EditURL       string                         `json:"edit_url"`
	IsUnpublished bool                           `json:"is_unpublished"`
	Position      interface{}                    `json:"position"`
	Excerpt       string                         `json:"excerpt"`
	IsCurrent     bool                           `json:"is_current"`
}

type ConfluenceServerParentComment struct {
	CreatedAt   int64                `json:"created_at"`
	Title       *string              `json:"title"`
	Version     int                  `json:"version"`
	CreatedBy   ConfluenceServerUser `json:"created_by"`
	URL         string               `json:"url"`
	Content     string               `json:"content"`
	Labels      []string             `json:"labels"`
	HTMLContent string               `json:"html_content"`
	ContentType string               `json:"content_type"`
	UpdatedAt   int64                `json:"updated_at"`
	UpdatedBy   ConfluenceServerUser `json:"updated_by"`
	ID          string               `json:"id"`
	Excerpt     string               `json:"excerpt"`
}

type ConfluenceServerComment struct {
	ParentComment    *ConfluenceServerParentComment `json:"parent"`
	DisplayTitle     string                         `json:"display_title"`
	IsInlineComment  bool                           `json:"is_inline_comment"`
	CreatedAt        int64                          `json:"created_at"`
	ThreadChangeDate int64                          `json:"thread_change_date"`
	DescendantsCount int                            `json:"descendants_count"`
	Title            *string                        `json:"title"`
	Version          int                            `json:"version"`
	CreatedBy        ConfluenceServerUser           `json:"created_by"`
	URL              string                         `json:"url"`
	Content          string                         `json:"content"`
	Labels           []string                       `json:"labels"`
	HTMLContent      string                         `json:"html_content"`
	Depth            int                            `json:"depth"`
	ContentType      string                         `json:"content_type"`
	UpdatedAt        int64                          `json:"updated_at"`
	UpdatedBy        ConfluenceServerUser           `json:"updated_by"`
	ID               int64                          `json:"id"`
	Excerpt          string                         `json:"excerpt"`
	Status           string                         `json:"status"`
}

type ConfluenceServerBlogPost struct {
	CreatedAt   int64                `json:"created_at"`
	Title       string               `json:"title"`
	Version     int                  `json:"version"`
	CreatedBy   ConfluenceServerUser `json:"created_by"`
	URL         string               `json:"url"`
	Content     string               `json:"content"`
	Labels      []string             `json:"labels"`
	HTMLContent string               `json:"html_content"`
	ContentType string               `json:"content_type"`
	UpdatedAt   int64                `json:"updated_at"`
	UpdatedBy   ConfluenceServerUser `json:"updated_by"`
	ID          string               `json:"id"`
	Excerpt     string               `json:"excerpt"`
}

type ConfluenceServerEvent struct {
	VersionComment string                    `json:"version_comment"`
	IsMinorEdit    bool                      `json:"is_minor_edit"`
	Creator        ConfluenceServerUser      `json:"creator"`
	ContentType    string                    `json:"content_type"`
	BaseURL        string                    `json:"base_url"`
	ContentURL     string                    `json:"content_url"`
	ContainerType  string                    `json:"container_type"`
	Comment        *ConfluenceServerComment  `json:"comment"`
	Page           *ConfluenceServerPage     `json:"page"`
	Blog           *ConfluenceServerBlogPost `json:"blog"`
	Event          string                    `json:"event"`
	Excerpt        string                    `json:"excerpt"`
	User           *ConfluenceServerUser     `json:"user"`
	Space          ConfluenceServerSpace     `json:"space"`
	Timestamp      int64                     `json:"timestamp"`
	UserKey        string                    `json:"userKey"`
}

type CommentPayload struct {
	ID int64 `json:"id"`
}

type PagePayload struct {
	ID int64 `json:"id"`
}

type SpacePayload struct {
	ID       int64  `json:"id"`
	SpaceKey string `json:"spaceKey"`
}

type ConfluenceServerWebhookPayload struct {
	Timestamp int64          `json:"timestamp"`
	Event     string         `json:"event"`
	UserKey   string         `json:"userKey"`
	Comment   CommentPayload `json:"comment"`
	Page      PagePayload    `json:"page"`
	Space     SpacePayload   `json:"space"`
}

func ConfluenceServerEventFromJSON(body []byte) *ConfluenceServerWebhookPayload {
	var confluenceServerWebhookPayload *ConfluenceServerWebhookPayload
	err := json.Unmarshal(body, &confluenceServerWebhookPayload)
	if err != nil {
		config.Mattermost.LogError("Error occurred while unmarshalling confluence server webhook payload.", "Error", err.Error())
	}
	return confluenceServerWebhookPayload
}

func (e *ConfluenceServerEvent) GetUserDisplayName(withLink bool) string {
	name := utils.Someone
	if e.User == nil {
		return name
	}

	if strings.TrimSpace(e.User.FullName) != "" {
		name = strings.TrimSpace(e.User.FullName)
	} else if strings.TrimSpace(e.User.Username) != "" {
		name = strings.TrimSpace(e.User.Username)
	}

	if withLink && e.User.URL != "" {
		name = fmt.Sprintf("[%s](%s)", name, e.User.URL)
	}

	return name
}

func (e *ConfluenceServerEvent) GetUserFirstName() string {
	return strings.Split(e.GetUserDisplayName(false), " ")[0]
}

func (e *ConfluenceServerEvent) GetSpaceDisplayName(withLink bool) string {
	name := e.Space.Key
	if strings.TrimSpace(e.Space.Name) != "" {
		name = strings.TrimSpace(e.Space.Name)
	}

	if withLink && e.Space.URL != "" {
		name = fmt.Sprintf("[%s](%s)", name, e.Space.URL)
	}

	return name
}

func (e *ConfluenceServerEvent) GetPageDisplayName(withLink bool) string {
	if e.Page == nil {
		return ""
	}

	name := e.Page.Title
	if withLink && e.Page.TinyURL != "" {
		name = fmt.Sprintf("[%s](%s)", name, e.Page.TinyURL)
	}

	return name
}

func (e *ConfluenceServerEvent) GetBlogDisplayName(withLink bool) string {
	if e.Blog == nil {
		return ""
	}

	name := e.Blog.Title
	if withLink && e.Blog.URL != "" {
		name = fmt.Sprintf("[%s](%s)", name, e.Blog.URL)
	}

	return name
}

func (e *ConfluenceServerEvent) GetCommentPageOrBlogDisplayName(withLink bool) string {
	commentedOn := e.GetPageDisplayName(withLink)
	if commentedOn == "" {
		commentedOn = e.GetBlogDisplayName(withLink)
	}
	return commentedOn
}

func (e ConfluenceServerEvent) GetURL() string {
	return e.BaseURL
}

func (e ConfluenceServerEvent) GetSpaceKey() string {
	return e.Space.Key
}

func (e ConfluenceServerEvent) GetPageID() string {
	if e.Page != nil {
		return e.Page.ID
	}
	return ""
}
