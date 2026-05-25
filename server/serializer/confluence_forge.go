// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package serializer

import (
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

// ForgeEvent is the wire shape of a Confluence Forge trigger payload.
// Docs: https://developer.atlassian.com/platform/forge/events-reference/confluence/
// BaseURL is injected by the poller after decode since Forge payloads do not
// carry the site URL.
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

// ForgeContent is the `content` field for page, blog post, and comment events.
// Container is populated only for comments and points at the parent page.
// The Forge events ref's TypeScript declares CommentContainer.id as a number,
// but every example payload in the same doc emits it as a quoted string; we
// follow the payloads.
type ForgeContent struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Title     string        `json:"title"`
	Status    string        `json:"status"`
	Space     ForgeSpace    `json:"space"`
	Container *ForgeContent `json:"container,omitempty"`
}

type ForgeSpace struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

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

// GetPageID returns the parent page id for comment events so that per-page
// subscription lookups match.
func (e *ForgeEvent) GetPageID() string {
	if e.Content == nil {
		return ""
	}
	if e.Content.Type == "comment" && e.Content.Container != nil {
		return e.Content.Container.ID
	}
	return e.Content.ID
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

	// Use the space name for display (not the key) so personal-space keys
	// like ~accountId aren't parsed as Mattermost channel mentions.
	spaceDisplay := e.Content.Space.Name
	if spaceDisplay == "" {
		spaceDisplay = e.Content.Space.Key
	}
	var message string

	switch eventType {
	case PageCreatedEvent:
		message = fmt.Sprintf(forgePageCreateMessage, e.Content.Title, e.pageURL(e.Content.ID), spaceDisplay)
	case PageUpdatedEvent:
		message = fmt.Sprintf(forgePageUpdateMessage, e.Content.Title, e.pageURL(e.Content.ID), spaceDisplay)
	case PageTrashedEvent:
		message = fmt.Sprintf(forgePageTrashMessage, e.Content.Title, e.pageURL(e.Content.ID), spaceDisplay)
	case PageRestoredEvent:
		message = fmt.Sprintf(forgePageRestoreMessage, e.Content.Title, e.pageURL(e.Content.ID), spaceDisplay)
	case PageRemovedEvent:
		message = fmt.Sprintf(forgePageDeleteMessage, e.Content.Title, spaceDisplay)

	case CommentCreatedEvent, CommentUpdatedEvent, CommentRemovedEvent:
		if e.Content.Container == nil {
			return nil
		}
		parentURL := e.pageURL(e.Content.Container.ID)
		commentURL := parentURL
		if parentURL != "" && e.Content.ID != "" {
			commentURL = fmt.Sprintf("%s?focusedCommentId=%s", parentURL, e.Content.ID)
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
