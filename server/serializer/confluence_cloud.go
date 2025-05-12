package serializer

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const (
	confluenceCloudPageCreateMessage    = "A new page titled [%s](%s) was created in the **%s** space."
	confluenceCloudPageUpdateMessage    = "A page titled [%s](%s) was updated in the **%s** space."
	confluenceCloudPageDeleteMessage    = "A page titled **%s** was removed from the **%s** space."
	confluenceCloudCommentCreateMessage = "A new [comment](%s) was posted on the [%s](%s) page."
	confluenceCloudCommentUpdateMessage = "A [comment](%s) was updated on the [%s](%s) page."
	confluenceCloudCommentDeleteMessage = "A comment was deleted from the [%s](%s) page."
)

type ConfluenceCloudEvent struct {
	UserAccountID string   `json:"userAccountId"`
	AccountType   string   `json:"accountType"`
	UpdateTrigger string   `json:"updateTrigger"`
	Timestamp     int      `json:"timestamp"`
	Comment       *Comment `json:"comment"`
	Page          *Page    `json:"page"`
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
}

type ParentComment struct {
	ID string `json:"id"`
}

func ConfluenceCloudEventFromJSON(data io.Reader) (*ConfluenceCloudEvent, error) {
	var confluenceCloudEvent ConfluenceCloudEvent
	if err := json.NewDecoder(data).Decode(&confluenceCloudEvent); err != nil {
		config.Mattermost.LogError("Unable to decode JSON for ConfluenceServerEvent.", "Error", err.Error())
		return nil, err
	}
	return &confluenceCloudEvent, nil
}

func (e ConfluenceCloudEvent) GetNotificationPost(eventType string) *model.Post {
	message := ""
	page := e.Page
	comment := e.Comment
	switch eventType {
	case PageCreatedEvent:
		message = fmt.Sprintf(confluenceCloudPageCreateMessage, page.Title, page.Self, page.SpaceKey)
	case CommentCreatedEvent:
		message = fmt.Sprintf(confluenceCloudCommentCreateMessage, comment.Self, comment.Parent.Title, comment.Parent.Self)
	case PageUpdatedEvent:
		message = fmt.Sprintf(confluenceCloudPageUpdateMessage, page.Title, page.Self, page.SpaceKey)
	case CommentUpdatedEvent:
		message = fmt.Sprintf(confluenceCloudCommentUpdateMessage, comment.Self, comment.Parent.Title, comment.Parent.Self)
	case PageRemovedEvent:
		message = fmt.Sprintf(confluenceCloudPageDeleteMessage, page.Title, page.SpaceKey)
	case CommentRemovedEvent:
		message = fmt.Sprintf(confluenceCloudCommentDeleteMessage, comment.Parent.Title, comment.Parent.Self)
	default:
		return nil
	}

	post := &model.Post{
		UserId:  config.BotUserID,
		Type:    model.PostTypeDefault,
		Message: message,
	}
	return post
}

func (e ConfluenceCloudEvent) GetURL() string {
	if e.Comment != nil {
		return e.Comment.Self
	} else if e.Page != nil {
		return e.Page.Self
	}
	return ""
}

func (e ConfluenceCloudEvent) GetSpaceKey() string {
	if e.Comment != nil {
		return e.Comment.SpaceKey
	} else if e.Page != nil {
		return e.Page.SpaceKey
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
