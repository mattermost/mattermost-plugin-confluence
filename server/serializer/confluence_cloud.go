package serializer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

type ConfluenceCloudEvent struct {
	UserAccountID string   `json:"userAccountId"`
	AccountType   string   `json:"accountType"`
	UpdateTrigger string   `json:"updateTrigger"`
	Timestamp     int      `json:"timestamp"`
	Comment       *Comment `json:"comment"`
	Page          *Page    `json:"page"`
	Space         *Space   `json:"space"`
}

type Space struct {
	CreatorAccountID      string `json:"creatorAccountId"`
	SpaceKey              string `json:"key"`
	ModificationDate      int64  `json:"modificationDate"`
	LastModifierAccountID string `json:"lastModifierAccountId"`
	Self                  string `json:"self"`
	Title                 string `json:"title"`
	CreationDate          int    `json:"creationDate"`
}
type View struct {
	Value string `json:"value"`
}
type Body struct {
	View View `json:"view"`
}

type Page struct {
	CreatorAccountID      string `json:"creatorAccountId"`
	SpaceKey              string `json:"spaceKey"`
	ModificationDate      int64  `json:"modificationDate"`
	LastModifierAccountID string `json:"lastModifierAccountId"`
	Self                  string `json:"self"`
	ID                    int    `json:"id"`
	Title                 string `json:"title"`
	CreationDate          int    `json:"creationDate"`
	ContentTypes          string `json:"contentType"`
	Version               int    `json:"version"`
	Body                  Body   `json:"body"`
	UserName              string `json:"userName"`
	SpaceLink             string `json:"spaceLink"`
}

type Comment struct {
	CreatorAccountID      string         `json:"creatorAccountId"`
	SpaceKey              string         `json:"spaceKey"`
	ModificationDate      int64          `json:"modificationDate"`
	LastModifierAccountID string         `json:"lastModifierAccountId"`
	Self                  string         `json:"self"`
	ID                    int            `json:"id"`
	CreationDate          int            `json:"creationDate"`
	ContentTypes          string         `json:"contentType"`
	Version               int            `json:"version"`
	Parent                *Page          `json:"parent"`
	InReplyTo             *ParentComment `json:"inReplyTo"`
	Body                  Body           `json:"body"`
	UserName              string         `json:"userName"`
	SpaceLink             string         `json:"spaceLink"`
}

type ParentComment struct {
	ID string `json:"id"`
}

func ConfluenceCloudEventFromJSON(body []byte) *ConfluenceCloudEvent {
	var confluenceCloudEvent ConfluenceCloudEvent
	if err := json.Unmarshal(body, &confluenceCloudEvent); err != nil {
		config.Mattermost.LogError("Unable to decode JSON for ConfluenceCloudEvent.", "Error", err.Error())
	}
	return &confluenceCloudEvent
}

func (e ConfluenceCloudEvent) GetNotificationPost(eventType, baseURL, botUserID string) *model.Post {
	var attachment *model.SlackAttachment
	post := &model.Post{
		UserId: botUserID,
	}

	switch eventType {
	case PageCreatedEvent:
		message := fmt.Sprintf(ConfluencePageCreatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetSpaceDisplayNameForPageEvents(baseURL, true))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback:  message,
				Pretext:   message,
				Title:     e.Page.Title,
				TitleLink: e.Page.Self,
				Text:      fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), e.Page.Self),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluencePageCreatedWithoutBodyMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL, true), e.GetSpaceDisplayNameForPageEvents(baseURL, true))
		}

	case PageUpdatedEvent:
		message := fmt.Sprintf(ConfluencePageUpdatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL, true), e.GetSpaceDisplayNameForPageEvents(baseURL, true))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**What’s Changed?**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), e.Page.Self),
			}
		} else {
			post.Message = message
		}

	case PageTrashedEvent:
		post.Message = fmt.Sprintf(ConfluencePageTrashedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL, true), e.GetSpaceDisplayNameForPageEvents(baseURL, true))

	case PageRestoredEvent:
		post.Message = fmt.Sprintf(ConfluencePageRestoredMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL, true), e.GetSpaceDisplayNameForPageEvents(baseURL, true))

	case CommentCreatedEvent:
		message := fmt.Sprintf(ConfluenceCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL, true), e.GetSpaceDisplayNameForCommentEvents(baseURL, true))
		text := ""
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			text = fmt.Sprintf("**%s wrote:**\n> %s\n\n", e.GetUserDisplayNameForCommentEvents(), strings.TrimSpace(e.Comment.Body.View.Value))
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", text, e.Comment.Self),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluenceEmptyCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), e.Comment.Self, e.GetPageDisplayNameForCommentEvents(baseURL, true), e.GetSpaceDisplayNameForCommentEvents(baseURL, true))
		}

	case CommentUpdatedEvent:
		message := fmt.Sprintf(ConfluenceCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL, true), e.GetSpaceDisplayNameForCommentEvents(baseURL, true))
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**Updated Comment:**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Comment.Body.View.Value), e.Comment.Self),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluenceEmptyCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), e.Comment.Self, e.GetPageDisplayNameForCommentEvents(baseURL, true), e.GetSpaceDisplayNameForCommentEvents(baseURL, true))
		}

	case SpaceUpdatedEvent:
		post.Message = fmt.Sprintf(ConfluenceSpaceUpdatedMessage, e.Space.SpaceKey, e.Space.Self)
	default:
		return nil
	}

	if attachment != nil {
		model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	}
	return post
}

func (e ConfluenceCloudEvent) GetURL() string {
	if e.Comment != nil {
		return e.Comment.Self
	}
	if e.Page != nil {
		return e.Page.Self
	}
	if e.Space != nil {
		return e.Space.Self
	}
	return ""
}

func (e ConfluenceCloudEvent) GetSpaceKey() string {
	if e.Comment != nil {
		return e.Comment.SpaceKey
	}
	if e.Page != nil {
		return e.Page.SpaceKey
	}
	if e.Space != nil {
		return e.Space.SpaceKey
	}
	return ""
}

func (e ConfluenceCloudEvent) GetPageID() string {
	if e.Comment != nil {
		return strconv.Itoa(e.Comment.Parent.ID)
	} else if e.Page != nil {
		return strconv.Itoa(e.Page.ID)
	}
	return ""
}

func (e *ConfluenceCloudEvent) GetUserDisplayNameForCommentEvents() string {
	return utils.GetUsernameOrAnonymousName(e.Comment.UserName)
}

func (e *ConfluenceCloudEvent) GetUserDisplayNameForPageEvents() string {
	return utils.GetUsernameOrAnonymousName(e.Page.UserName)
}

func (e *ConfluenceCloudEvent) GetSpaceDisplayNameForCommentEvents(baseURL string, withLink bool) string {
	name := e.Comment.SpaceKey
	if withLink && e.Comment.SpaceLink != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Comment.SpaceLink)
	}
	return name
}

func (e *ConfluenceCloudEvent) GetSpaceDisplayNameForPageEvents(baseURL string, withLink bool) string {
	name := e.Page.SpaceKey
	if withLink && e.Page.SpaceLink != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Page.SpaceLink)
	}
	return name
}

func (e *ConfluenceCloudEvent) GetPageDisplayNameForPageEvents(baseURL string, withLink bool) string {
	if e.Page.Title == "" {
		return ""
	}
	name := e.Page.Title
	if withLink && e.Page.Self != "" {
		name = fmt.Sprintf("[%s](%s)", name, e.Page.Self)
	}
	return name
}

func (e *ConfluenceCloudEvent) GetPageDisplayNameForCommentEvents(baseURL string, withLink bool) string {
	if e.Comment.Parent.Title == "" {
		return ""
	}
	name := e.Comment.Parent.Title
	if withLink && e.Comment.Parent.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Comment.Parent.Self)
	}
	return name
}
