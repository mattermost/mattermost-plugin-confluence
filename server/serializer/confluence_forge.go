// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package serializer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
)

const (
	forgePageCreateMessage    = "A new page titled [%s](%s) was created in the **%s** space."
	forgePageUpdateMessage    = "A page titled [%s](%s) was updated in the **%s** space."
	forgePageTrashMessage     = "A page titled [%s](%s) was moved to trash from the **%s** space."
	forgePageRestoreMessage   = "A page titled [%s](%s) was restored to the **%s** space."
	forgePageDeleteMessage    = "A page titled **%s** was removed from the **%s** space."
	forgeCommentCreateMessage = "A new [comment](%s) was posted on the [%s](%s) page."
	forgeCommentUpdateMessage = "A [comment](%s) was updated on the [%s](%s) page."
	forgeCommentDeleteMessage = "A comment was deleted from the [%s](%s) page."
)

// Forge trigger payload. BaseURL is injected by the poller after decode
// since Forge payloads do not carry the site URL.
// Docs: https://developer.atlassian.com/platform/forge/events-reference/confluence/
type ForgeEvent struct {
	EventType     string        `json:"eventType"`
	AtlassianID   string        `json:"atlassianId"`
	UpdateTrigger string        `json:"updateTrigger,omitempty"`
	Context       ForgeContext  `json:"context"`
	Content       *ForgeContent `json:"content"`

	BaseURL string `json:"-"`
}

type ForgeContext struct {
	CloudID   string `json:"cloudId"`
	ModuleKey string `json:"moduleKey"`
}

// Container is populated only for comments; Extensions.Location is "footer"
// or "inline" on comment events.
type ForgeContent struct {
	ID         forgeID         `json:"id"`
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Status     string          `json:"status"`
	Space      ForgeSpace      `json:"space"`
	Container  *ForgeContent   `json:"container,omitempty"`
	Extensions ForgeExtensions `json:"extensions,omitempty"`
}

type ForgeExtensions struct {
	Location string `json:"location,omitempty"`
}

type ForgeSpace struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// forgeID decodes JSON values that may be either string or number into a string.
type forgeID string

func (f *forgeID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		*f = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*f = forgeID(s)
		return nil
	}
	*f = forgeID(string(trimmed))
	return nil
}

func (f forgeID) String() string { return string(f) }

func ForgeEventFromJSON(r io.Reader) (*ForgeEvent, error) {
	var evt ForgeEvent
	if err := json.NewDecoder(r).Decode(&evt); err != nil {
		config.Mattermost.LogError("Unable to decode Forge Confluence event", "Error", err.Error())
		return nil, err
	}
	return &evt, nil
}

func (e *ForgeEvent) GetURL() string {
	return e.BaseURL
}

func (e *ForgeEvent) GetSpaceKey() string {
	if e.Content == nil {
		return ""
	}
	if e.Content.Space.Key != "" {
		return e.Content.Space.Key
	}
	if e.Content.Container != nil {
		return e.Content.Container.Space.Key
	}
	return ""
}

// GetPageID returns the parent page id for comment events so per-page
// subscription lookups match.
func (e *ForgeEvent) GetPageID() string {
	if e.Content == nil {
		return ""
	}
	if e.Content.Type == "comment" && e.Content.Container != nil {
		return e.Content.Container.ID.String()
	}
	return e.Content.ID.String()
}

func (e *ForgeEvent) pageURL(pageID string) string {
	spaceKey := e.GetSpaceKey()
	if e.BaseURL == "" || spaceKey == "" || pageID == "" {
		return ""
	}
	return fmt.Sprintf("%s/spaces/%s/pages/%s", e.BaseURL, spaceKey, pageID)
}

func (e *ForgeEvent) GetNotificationPost(eventType string) *model.Post {
	if e.Content == nil {
		return nil
	}

	// Name not key: personal-space keys (~accountId) parse as MM channel mentions.
	spaceDisplay := e.Content.Space.Name
	if spaceDisplay == "" {
		spaceDisplay = e.Content.Space.Key
	}
	var message string

	contentID := e.Content.ID.String()

	switch eventType {
	case PageCreatedEvent:
		message = fmt.Sprintf(forgePageCreateMessage, e.Content.Title, e.pageURL(contentID), spaceDisplay)
	case PageUpdatedEvent:
		message = fmt.Sprintf(forgePageUpdateMessage, e.Content.Title, e.pageURL(contentID), spaceDisplay)
	case PageTrashedEvent:
		message = fmt.Sprintf(forgePageTrashMessage, e.Content.Title, e.pageURL(contentID), spaceDisplay)
	case PageRestoredEvent:
		message = fmt.Sprintf(forgePageRestoreMessage, e.Content.Title, e.pageURL(contentID), spaceDisplay)
	case PageRemovedEvent:
		message = fmt.Sprintf(forgePageDeleteMessage, e.Content.Title, spaceDisplay)

	case CommentCreatedEvent, CommentUpdatedEvent, CommentRemovedEvent:
		if e.Content.Container == nil {
			return nil
		}
		parentURL := e.pageURL(e.Content.Container.ID.String())
		commentURL := parentURL
		if parentURL != "" && contentID != "" {
			commentURL = fmt.Sprintf("%s?focusedCommentId=%s", parentURL, contentID)
		}
		switch eventType {
		case CommentCreatedEvent:
			message = fmt.Sprintf(forgeCommentCreateMessage, commentURL, e.Content.Container.Title, parentURL)
		case CommentUpdatedEvent:
			message = fmt.Sprintf(forgeCommentUpdateMessage, commentURL, e.Content.Container.Title, parentURL)
		case CommentRemovedEvent:
			message = fmt.Sprintf(forgeCommentDeleteMessage, e.Content.Container.Title, parentURL)
		}

	default:
		return nil
	}

	return &model.Post{
		UserId:  config.BotUserID,
		Type:    model.PostTypeDefault,
		Message: message,
	}
}

func (e *ForgeEvent) ActorAccountID() string {
	return e.AtlassianID
}

func (e *ForgeEvent) CommentLocation() string {
	if e.Content == nil {
		return ""
	}
	return e.Content.Extensions.Location
}

// MentionPageContext returns the parent page's title and URL: the page itself
// for page events, Content.Container for comment events.
func (e *ForgeEvent) MentionPageContext() (string, string) {
	if e.Content == nil {
		return "", ""
	}
	if e.Content.Type == "comment" && e.Content.Container != nil {
		return e.Content.Container.Title, e.pageURL(e.Content.Container.ID.String())
	}
	return e.Content.Title, e.pageURL(e.Content.ID.String())
}
