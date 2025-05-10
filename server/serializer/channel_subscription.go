package serializer

import (
	"encoding/json"
	"strings"
)

const (
	CommentCreatedEvent   = "comment_created"
	CommentUpdatedEvent   = "comment_updated"
	CommentRemovedEvent   = "comment_removed"
	PageCreatedEvent      = "page_created"
	PageUpdatedEvent      = "page_updated"
	PageTrashedEvent      = "page_trashed"
	PageRestoredEvent     = "page_restored"
	PageRemovedEvent      = "page_removed"
	SubscriptionTypeSpace = "space_subscription"
	SpaceUpdatedEvent     = "space_updated"
	SubscriptionTypePage  = "page_subscription"

	aliasAlreadyExist       = "a subscription with the same name already exists in this channel"
	urlSpaceKeyAlreadyExist = "a subscription with the same url and space key already exists in this channel"
	urlPageIDAlreadyExist   = "a subscription with the same url and page id already exists in this channel"
)

var eventDisplayName = map[string]string{
	CommentCreatedEvent: "Comment Create",
	CommentUpdatedEvent: "Comment Update",
	CommentRemovedEvent: "Comment Remove",
	PageCreatedEvent:    "Page Create",
	PageUpdatedEvent:    "Page Update",
	PageTrashedEvent:    "Page Trash",
	PageRestoredEvent:   "Page Restore",
	PageRemovedEvent:    "Page Remove",
}

type Subscription interface {
	Add(*Subscriptions)
	Remove(*Subscriptions)
	Edit(*Subscriptions)
	Name() string
	GetAlias() string
	GetFormattedSubscription() string
	IsValid() error
	ValidateSubscription(*Subscriptions) error
}

type BaseSubscription struct {
	Alias     string   `json:"alias"`
	BaseURL   string   `json:"baseURL"`
	Events    []string `json:"events"`
	ChannelID string   `json:"channelID"`
	Type      string   `json:"subscriptionType"`
}

type StringSubscription map[string]Subscription
type StringArrayMap map[string][]string

type Subscriptions struct {
	ByChannelID   map[string]StringSubscription
	ByURLPageID   map[string]StringArrayMap
	ByURLSpaceKey map[string]StringArrayMap
}

func (s *Subscriptions) EnsureDefaults() {
	if s.ByChannelID == nil {
		s.ByChannelID = make(map[string]StringSubscription)
	}
	if s.ByURLPageID == nil {
		s.ByURLPageID = make(map[string]StringArrayMap)
	}
	if s.ByURLSpaceKey == nil {
		s.ByURLSpaceKey = make(map[string]StringArrayMap)
	}
}

func NewSubscriptions() *Subscriptions {
	return &Subscriptions{
		ByChannelID:   map[string]StringSubscription{},
		ByURLPageID:   map[string]StringArrayMap{},
		ByURLSpaceKey: map[string]StringArrayMap{},
	}
}

func SubscriptionsFromJSON(bytes []byte) (*Subscriptions, error) {
	var subs *Subscriptions
	if len(bytes) != 0 {
		unmarshalErr := json.Unmarshal(bytes, &subs)
		if unmarshalErr != nil {
			return nil, unmarshalErr
		}
	} else {
		subs = NewSubscriptions()
	}
	return subs, nil
}

func FormattedSubscriptionList(channelSubscriptions StringSubscription) string {
	var pageSubscriptions, spaceSubscriptions, list string
	pageSubscriptionsHeader := "| Name | Base Url | Page Id | Events|\n| :----|:--------| :--------| :-----|"
	spaceSubscriptionsHeader := "| Name | Base Url | Space Key | Events|\n| :----|:--------| :--------| :-----|"
	for _, sub := range channelSubscriptions {
		if sub.Name() == SubscriptionTypePage {
			pageSubscriptions += sub.GetFormattedSubscription()
		} else if sub.Name() == SubscriptionTypeSpace {
			spaceSubscriptions += sub.GetFormattedSubscription()
		}
	}
	if spaceSubscriptions != "" {
		list = "#### Space Subscriptions \n" + spaceSubscriptionsHeader + spaceSubscriptions
	}
	if spaceSubscriptions != "" && pageSubscriptions != "" {
		list += "\n\n"
	}
	if pageSubscriptions != "" {
		list += "#### Page Subscriptions \n" + pageSubscriptionsHeader + pageSubscriptions
	}
	return list
}

func (s StringSubscription) GetInsensitiveCase(key string) (Subscription, bool) {
	key = strings.ToLower(key)
	for k, v := range s {
		k = strings.ToLower(k)
		if key == k {
			return v, true
		}
	}
	return nil, false
}
