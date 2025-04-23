package serializer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const (
	ConfluenceContentTypePage                     = "page"
	ConfluenceContentTypeBlogPost                 = "blogpost"
	ConfluenceContentTypeComment                  = "comment"
	confluenceServerPageCreatedMessage            = "%s published a new page in %s."
	confluenceServerPageCreatedWithoutBodyMessage = "%s published a new page %s in %s."
	confluenceServerPageUpdatedMessage            = "%s updated %s in %s."
	confluenceServerPageTrashedMessage            = "%s trashed %s in %s."
	confluenceServerPageRestoredMessage           = "%s restored %s in %s."
	confluenceServerPageRemovedMessage            = "%s removed **%s** in %s."

	confluenceServerCommentCreatedMessage      = "%s commented on %s in %s."
	confluenceServerEmptyCommentCreatedMessage = "%s [commented](%s) on %s in %s."
	confluenceServerCommentReplyCreatedMessage = "%s replied to a comment on %s in %s."
	confluenceServerCommentUpdatedMessage      = "%s updated a comment on %s in %s."
	confluenceServerEmptyCommentUpdatedMessage = "%s updated a [comment](%s) on %s in %s."
	confluenceServerCommentRemovedMessage      = "%s removed a comment on %s in %s."
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
	ID               string                         `json:"id"`
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

func ConfluenceServerEventFromJSON(data io.Reader) *ConfluenceServerEvent {
	var confluenceServerEvent ConfluenceServerEvent
	if err := json.NewDecoder(data).Decode(&confluenceServerEvent); err != nil {
		config.Mattermost.LogError("Unable to decode JSON for ConfluenceServerEvent.", "Error", err.Error())
	}
	return &confluenceServerEvent
}

func (e *ConfluenceServerEvent) GetUserDisplayName(withLink bool) string {
	name := "Someone"
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

func (e ConfluenceServerEvent) GetNotificationPost(_ string) *model.Post {
	var attachment *model.SlackAttachment
	post := &model.Post{
		UserId: config.BotUserID,
	}

	switch e.Event {
	case PageCreatedEvent:
		message := fmt.Sprintf(confluenceServerPageCreatedMessage, e.GetUserDisplayName(true), e.GetSpaceDisplayName(true))
		if strings.TrimSpace(e.Page.Excerpt) != "" {
			attachment = &model.SlackAttachment{
				Fallback:  message,
				Pretext:   message,
				Title:     e.Page.Title,
				TitleLink: e.Page.TinyURL,
				Text:      fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Excerpt), e.Page.TinyURL),
			}
		} else {
			post.Message = fmt.Sprintf(confluenceServerPageCreatedWithoutBodyMessage, e.GetUserDisplayName(true), e.GetPageDisplayName(true), e.GetSpaceDisplayName(true))
		}

	case PageUpdatedEvent:
		message := fmt.Sprintf(confluenceServerPageUpdatedMessage, e.GetUserDisplayName(true), e.GetPageDisplayName(true), e.GetSpaceDisplayName(true))
		if strings.TrimSpace(e.VersionComment) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**What’s Changed?**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.VersionComment), e.Page.TinyURL),
			}
		} else {
			post.Message = message
		}

	case PageTrashedEvent:
		post.Message = fmt.Sprintf(confluenceServerPageTrashedMessage, e.GetUserDisplayName(true), e.GetPageDisplayName(true), e.GetSpaceDisplayName(true))

	case PageRestoredEvent:
		post.Message = fmt.Sprintf(confluenceServerPageRestoredMessage, e.GetUserDisplayName(true), e.GetPageDisplayName(true), e.GetSpaceDisplayName(true))

	case PageRemovedEvent:
		// No link for page since the page was removed
		post.Message = fmt.Sprintf(confluenceServerPageRemovedMessage, e.GetUserDisplayName(true), e.GetPageDisplayName(false), e.GetSpaceDisplayName(true))

	case CommentCreatedEvent:
		message := fmt.Sprintf(confluenceServerCommentCreatedMessage, e.GetUserDisplayName(true), e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))

		text := ""
		if strings.TrimSpace(e.Comment.Excerpt) != "" {
			text += fmt.Sprintf("**%s wrote:**\n> %s\n\n", e.GetUserFirstName(), strings.TrimSpace(e.Comment.Excerpt))
		}
		if e.Comment.ParentComment != nil && strings.TrimSpace(e.Comment.ParentComment.Excerpt) != "" {
			message = fmt.Sprintf(confluenceServerCommentReplyCreatedMessage, e.GetUserDisplayName(true), e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))
			text += fmt.Sprintf("**In Reply to:**\n> %s\n", strings.TrimSpace(e.Comment.ParentComment.Excerpt))
		}

		if text != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", text, e.Comment.URL),
			}
		} else {
			post.Message = fmt.Sprintf(confluenceServerEmptyCommentCreatedMessage, e.GetUserDisplayName(true), e.Comment.URL, e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))
		}

	case CommentUpdatedEvent:
		message := fmt.Sprintf(confluenceServerCommentUpdatedMessage, e.GetUserDisplayName(true), e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))
		if strings.TrimSpace(e.Comment.Excerpt) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**Updated Comment:**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Comment.Excerpt), e.Comment.URL),
			}
		} else {
			post.Message = fmt.Sprintf(confluenceServerEmptyCommentUpdatedMessage, e.GetUserDisplayName(true), e.Comment.URL, e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))
		}

	case CommentRemovedEvent:
		// No link since the comment was removed.
		message := fmt.Sprintf(confluenceServerCommentRemovedMessage, e.GetUserDisplayName(true), e.GetCommentPageOrBlogDisplayName(true), e.GetSpaceDisplayName(true))
		if strings.TrimSpace(e.Comment.Excerpt) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**Deleted Comment:**\n> %s", strings.TrimSpace(e.Comment.Excerpt)),
			}
		} else {
			post.Message = message
		}

	default:
		return nil
	}

	if attachment != nil {
		model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	}

	return post
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
