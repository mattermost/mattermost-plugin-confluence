package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

const (
	ConfluencePageCreatedMessage            = "%s published a new page in %s."
	ConfluencePageCreatedWithoutBodyMessage = "%s published a new page %s in %s."
	ConfluencePageUpdatedMessage            = "%s updated %s in %s."
	ConfluencePageTrashedMessage            = "%s trashed %s in %s."
	ConfluencePageRestoredMessage           = "%s restored %s in %s."
	ConfluenceCommentCreatedMessage         = "%s commented on %s in %s."
	ConfluenceEmptyCommentCreatedMessage    = "%s [commented](%s) on %s in %s."
	ConfluenceCommentUpdatedMessage         = "%s updated a comment on %s in %s."
	ConfluenceEmptyCommentUpdatedMessage    = "%s updated a [comment](%s) on %s in %s."
	ConfluenceSpaceUpdatedMessage           = "A space titled [%s](%s) was updated."
)

func (e ConfluenceServerEvent) GetSpaceKey() string {
	if e.Space == nil {
		return ""
	}

	return e.Space.Key
}

func (e ConfluenceServerEvent) GetURL() string {
	return e.BaseURL
}

func (e ConfluenceServerEvent) GetCommentSpaceKey() string {
	if e.Comment == nil {
		return ""
	}

	return e.Comment.Space.Key
}

func (e ConfluenceServerEvent) GetCommentContainerID() string {
	if e.Comment == nil {
		return ""
	}

	return e.Comment.Container.ID
}

func (e ConfluenceServerEvent) GetPageSpaceKey() string {
	if e.Page == nil {
		return ""
	}

	return e.Page.Space.Key
}

func (e ConfluenceServerEvent) GetPageID() string {
	if e.Page == nil {
		return ""
	}

	return e.Page.ID
}

func (e *ConfluenceServerEvent) GetUserDisplayNameForCommentEvents() string {
	if e.Comment == nil {
		return ""
	}

	return util.GetUsernameOrAnonymousName(e.Comment.History.CreatedBy.Username)
}

func (e *ConfluenceServerEvent) GetUserDisplayNameForPageEvents() string {
	if e.Page == nil {
		return ""
	}

	return util.GetUsernameOrAnonymousName(e.Page.History.CreatedBy.Username)
}

func (e *ConfluenceServerEvent) GetSpaceDisplayNameForCommentEvents(baseURL string) string {
	if e.Comment == nil {
		return ""
	}

	name := e.Comment.Space.Key
	if strings.TrimSpace(e.Comment.Space.Name) != "" {
		name = strings.TrimSpace(e.Comment.Space.Name)
	}
	if e.Comment.Space.Links.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Comment.Space.Links.Self)
	}
	return name
}

func (e *ConfluenceServerEvent) GetSpaceDisplayNameForPageEvents(baseURL string) string {
	if e.Page == nil {
		return ""
	}

	name := e.Page.Space.Key
	if strings.TrimSpace(e.Page.Space.Name) != "" {
		name = strings.TrimSpace(e.Page.Space.Name)
	}
	if e.Page.Space.Links.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Page.Space.Links.Self)
	}
	return name
}

func (e *ConfluenceServerEvent) GetPageDisplayNameForPageEvents(baseURL string) string {
	if e.Page == nil || e.Page.Title == "" {
		return ""
	}

	name := e.Page.Title
	if e.Page.Links.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Page.Links.Self)
	}
	return name
}

func (e *ConfluenceServerEvent) GetPageDisplayNameForCommentEvents(baseURL string) string {
	if e.Comment == nil || e.Comment.Container.Title == "" {
		return ""
	}

	name := e.Comment.Container.Title
	if e.Comment.Container.Links.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Comment.Container.Links.Self)
	}
	return name
}

func (e ConfluenceServerEvent) GetNotificationPost(eventType, baseURL, botUserID string) *model.Post {
	var attachment *model.SlackAttachment
	post := &model.Post{
		UserId: botUserID,
	}

	switch eventType {
	case serializer.PageCreatedEvent, serializer.PageUpdatedEvent, serializer.PageTrashedEvent, serializer.PageRestoredEvent:
		if e.Page == nil {
			return nil
		}
	case serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent:
		if e.Comment == nil {
			return nil
		}
	case serializer.SpaceUpdatedEvent:
		if e.Space == nil {
			return nil
		}
	default:
		return nil
	}

	switch eventType {
	case serializer.PageCreatedEvent:
		message := fmt.Sprintf(ConfluencePageCreatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetSpaceDisplayNameForPageEvents(baseURL))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback:  message,
				Pretext:   message,
				Title:     e.Page.Title,
				TitleLink: fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self),
				Text:      fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluencePageCreatedWithoutBodyMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))
		}

	case serializer.PageUpdatedEvent:
		message := fmt.Sprintf(ConfluencePageUpdatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**What's Changed?**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self)),
			}
		} else {
			post.Message = message
		}

	case serializer.PageTrashedEvent:
		post.Message = fmt.Sprintf(ConfluencePageTrashedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))

	case serializer.PageRestoredEvent:
		post.Message = fmt.Sprintf(ConfluencePageRestoredMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))

	case serializer.CommentCreatedEvent:
		message := fmt.Sprintf(ConfluenceCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		text := ""
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			text = fmt.Sprintf("**%s wrote:**\n> %s\n\n", e.GetUserDisplayNameForCommentEvents(), strings.TrimSpace(e.Comment.Body.View.Value))
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", text, fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluenceEmptyCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		}

	case serializer.CommentUpdatedEvent:
		message := fmt.Sprintf(ConfluenceCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**Updated Comment:**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Comment.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(ConfluenceEmptyCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		}

	case serializer.SpaceUpdatedEvent:
		post.Message = fmt.Sprintf(ConfluenceSpaceUpdatedMessage, e.Space.Key, fmt.Sprintf("%s/%s", baseURL, e.Space.Links.Self))
	default:
		return nil
	}

	if attachment != nil {
		model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	}
	return post
}
