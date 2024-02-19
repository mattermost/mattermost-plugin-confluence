package serializer

import (
	"encoding/json"
	"errors"
	"fmt"
	url2 "net/url"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

type PageSubscription struct {
	PageID string `json:"pageID"`
	BaseSubscription
	UserID string
}

type OldSubscriptionForPage struct {
	PageSubscription
}

type OldPageSubscription struct {
	OldSubscription OldSubscriptionForPage `json:"oldSubscription"`
}

func (ps *PageSubscription) Add(s *Subscriptions) {
	if _, valid := s.ByChannelID[ps.ChannelID]; !valid {
		s.ByChannelID[ps.ChannelID] = make(StringSubscription)
	}
	s.ByChannelID[ps.ChannelID][ps.Alias] = ps
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	if _, ok := s.ByURLPageID[key]; !ok {
		s.ByURLPageID[key] = make(map[string]StringArrayMap)
	}
	if _, ok := s.ByURLPageID[key][ps.ChannelID]; !ok {
		s.ByURLPageID[key][ps.ChannelID] = make(map[string][]string)
	}

	s.ByURLPageID[key][ps.ChannelID][ps.UserID] = ps.Events

	if s.BySpaceID == nil {
		s.BySpaceID = make(map[string]string)
	}
}

func (ps *PageSubscription) Remove(s *Subscriptions) {
	delete(s.ByChannelID[ps.ChannelID], ps.Alias)
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	delete(s.ByURLPageID[key], ps.ChannelID)
}

func (ps *PageSubscription) GetChannelID() string {
	return ps.BaseSubscription.ChannelID
}

func (ps *PageSubscription) Edit(s *Subscriptions) {
	ps.Remove(s)
	ps.Add(s)
}

func (ps *PageSubscription) Name() string {
	return SubscriptionTypePage
}

func (ps *PageSubscription) GetAlias() string {
	return ps.Alias
}

func (ps *PageSubscription) GetUserID() string {
	return ps.UserID
}

func (ps *PageSubscription) GetConfluenceURL() string {
	return ps.GetSubscription().BaseURL
}

func (ps *PageSubscription) GetEvents() []string {
	return ps.Events
}

func (ps *PageSubscription) GetSpaceKeyOrPageID() string {
	return ps.PageID
}

func (ps *PageSubscription) GetFormattedSubscription() string {
	var events []string
	for _, event := range ps.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|", ps.Alias, ps.BaseURL, ps.PageID, strings.Join(events, ", "))
}

func (ps *PageSubscription) GetOldFormattedSubscription() string {
	var events []string
	for _, event := range ps.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|%s|", ps.Alias, ps.BaseURL, ps.PageID, ps.ChannelID, strings.Join(events, ", "))
}

func (ps *PageSubscription) IsValid() error {
	if ps.Alias == "" {
		return errors.New("subscription name can not be empty")
	}
	if ps.BaseURL == "" {
		return errors.New("base url can not be empty")
	}
	if _, err := url2.Parse(ps.BaseURL); err != nil {
		return errors.New("enter a valid url")
	}
	if ps.PageID == "" {
		return errors.New("page id can not be empty")
	}
	if ps.ChannelID == "" {
		return errors.New("channel id can not be empty")
	}
	return nil
}

func PageSubscriptionFromJSON(body []byte) (*PageSubscription, error) {
	var ps PageSubscription
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, err
	}
	return &ps, nil
}

func OldPageSubscriptionFromJSON(body []byte) (*PageSubscription, error) {
	var ps OldPageSubscription
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, err
	}
	return &ps.OldSubscription.PageSubscription, nil
}

func (ps *PageSubscription) UpdateUserID(userID string) *PageSubscription {
	ps.UserID = userID
	return ps
}

func (ps *PageSubscription) GetSubscription() *PageSubscription {
	return &PageSubscription{
		PageID:           ps.PageID,
		BaseSubscription: ps.BaseSubscription,
		UserID:           ps.UserID,
	}
}

func (ps *PageSubscription) ValidateSubscription(subs *Subscriptions) error {
	if err := ps.IsValid(); err != nil {
		return err
	}
	if channelSubscriptions, valid := subs.ByChannelID[ps.ChannelID]; valid {
		if _, ok := channelSubscriptions[ps.Alias]; ok {
			return errors.New(AliasAlreadyExist)
		}
	}
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	if urlPageIDSubscriptions, valid := subs.ByURLPageID[key]; valid {
		if _, ok := urlPageIDSubscriptions[ps.ChannelID]; ok {
			return errors.New(URLPageIDAlreadyExist)
		}
	}
	return nil
}
