package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

// #nosec G101 -- This is a false positive. The below line is not a hardcoded credential
const webhookSecretHeader = "X-Hub-Signature"

func (p *Plugin) handleConfluenceServerWebhook(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	userID := params[ParamUserID]
	instance, err := p.getInstanceFromURL(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status, err := verifyWebHookSecret(p.conf.Secret, r.Header.Get(webhookSecretHeader), body)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	event := serializer.ConfluenceServerEventFromJSON(body)

	var spaceKey string
	if strings.Contains(event.Event, Space) {
		spaceKey, err = service.GetSubscriptionBySpaceID(strconv.FormatInt(event.Space.ID, 10))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		event.Space.SpaceKey = spaceKey
	}
	eventData, err := p.GetEventData(event, instance, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	eventData.BaseURL = instance.GetURL()

	notification := getNotification(p)

	notification.SendConfluenceNotifications(eventData, event.Event, p.conf.botUserID, ServerInstance, userID)

	w.Header().Set("Content-Type", "application/json")
	ReturnStatusOK(w)
}

func (p *Plugin) GetEventData(webhookPayload *serializer.ConfluenceServerWebhookPayload, instance Instance, userID string) (*ConfluenceServerEvent, error) {
	conn, err := p.userStore.LoadConnection(instance.GetID(), types.ID(userID))
	if err != nil {
		p.API.LogError("Error in loading connection.", "Error", err.Error())
		return nil, err
	}

	client, err := instance.(*serverInstance).GetClient(conn)
	if err != nil {
		p.API.LogError("Error occurred while fetching client.", "Error", err.Error())
		return nil, err
	}

	eventData, err := client.(*confluenceServerClient).GetEventData(webhookPayload)
	if err != nil {
		p.API.LogError("Error occurred while fetching event data.", "Error", err.Error())
		return nil, err
	}

	return eventData, nil
}

func (e ConfluenceServerEvent) GetURL() string {
	return e.BaseURL
}

func (e ConfluenceServerEvent) GetCommentSpaceKey() string {
	return e.Comment.Space.Key
}

func (e ConfluenceServerEvent) GetCommentContainerID() string {
	return e.Comment.Container.ID
}

func (e ConfluenceServerEvent) GetPageSpaceKey() string {
	return e.Page.Space.Key
}

func (e ConfluenceServerEvent) GetPageID() string {
	return e.Page.ID
}

func (e ConfluenceServerEvent) GetSpaceKey() string {
	return e.Space.Key
}

func (e *ConfluenceServerEvent) GetUserDisplayNameForCommentEvents() string {
	return utils.GetUsernameOrAnonymousName(e.Comment.History.CreatedBy.Username)
}

func (e *ConfluenceServerEvent) GetUserDisplayNameForPageEvents() string {
	return utils.GetUsernameOrAnonymousName(e.Page.History.CreatedBy.Username)
}

func (e *ConfluenceServerEvent) GetSpaceDisplayNameForCommentEvents(baseURL string) string {
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
	if e.Page.Title == "" {
		return ""
	}

	name := e.Page.Title
	if e.Page.Links.Self != "" {
		name = fmt.Sprintf("[%s](%s/%s)", name, baseURL, e.Page.Links.Self)
	}
	return name
}

func (e *ConfluenceServerEvent) GetPageDisplayNameForCommentEvents(baseURL string) string {
	if e.Comment.Container.Title == "" {
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
	case serializer.PageCreatedEvent:
		message := fmt.Sprintf(serializer.ConfluencePageCreatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetSpaceDisplayNameForPageEvents(baseURL))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback:  message,
				Pretext:   message,
				Title:     e.Page.Title,
				TitleLink: fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self),
				Text:      fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(serializer.ConfluencePageCreatedWithoutBodyMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))
		}

	case serializer.PageUpdatedEvent:
		message := fmt.Sprintf(serializer.ConfluencePageUpdatedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))
		if strings.TrimSpace(e.Page.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**What’s Changed?**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Page.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Page.Links.Self)),
			}
		} else {
			post.Message = message
		}

	case serializer.PageTrashedEvent:
		post.Message = fmt.Sprintf(serializer.ConfluencePageTrashedMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))

	case serializer.PageRestoredEvent:
		post.Message = fmt.Sprintf(serializer.ConfluencePageRestoredMessage, e.GetUserDisplayNameForPageEvents(), e.GetPageDisplayNameForPageEvents(baseURL), e.GetSpaceDisplayNameForPageEvents(baseURL))

	case serializer.CommentCreatedEvent:
		message := fmt.Sprintf(serializer.ConfluenceCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		text := ""
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			text = fmt.Sprintf("**%s wrote:**\n> %s\n\n", e.GetUserDisplayNameForCommentEvents(), strings.TrimSpace(e.Comment.Body.View.Value))
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("%s\n\n[**View in Confluence**](%s)", text, fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(serializer.ConfluenceEmptyCommentCreatedMessage, e.GetUserDisplayNameForCommentEvents(), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		}

	case serializer.CommentUpdatedEvent:
		message := fmt.Sprintf(serializer.ConfluenceCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		if strings.TrimSpace(e.Comment.Body.View.Value) != "" {
			attachment = &model.SlackAttachment{
				Fallback: message,
				Pretext:  message,
				Text:     fmt.Sprintf("**Updated Comment:**\n> %s\n\n[**View in Confluence**](%s)", strings.TrimSpace(e.Comment.Body.View.Value), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self)),
			}
		} else {
			post.Message = fmt.Sprintf(serializer.ConfluenceEmptyCommentUpdatedMessage, e.GetUserDisplayNameForCommentEvents(), fmt.Sprintf("%s/%s", baseURL, e.Comment.Links.Self), e.GetPageDisplayNameForCommentEvents(baseURL), e.GetSpaceDisplayNameForCommentEvents(baseURL))
		}

	case serializer.SpaceUpdatedEvent:
		post.Message = fmt.Sprintf(serializer.ConfluenceSpaceUpdatedMessage, e.Space.Key, fmt.Sprintf("%s/%s", baseURL, e.Space.Links.Self))
	default:
		return nil
	}

	if attachment != nil {
		model.ParseSlackAttachment(post, []*model.SlackAttachment{attachment})
	}
	return post
}
