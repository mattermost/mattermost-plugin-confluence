package serializer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	url2 "net/url"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

type PageSubscription struct {
	PageID string `json:"pageID"`
	BaseSubscription
}

func (ps PageSubscription) Add(s *Subscriptions) {
	s.EnsureDefaults()

	if _, valid := s.ByChannelID[ps.ChannelID]; !valid {
		s.ByChannelID[ps.ChannelID] = make(StringSubscription)
	}
	s.ByChannelID[ps.ChannelID][ps.Alias] = ps
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	if _, ok := s.ByURLPageID[key]; !ok {
		s.ByURLPageID[key] = make(map[string][]string)
	}
	s.ByURLPageID[key][ps.ChannelID] = ps.Events
}

func (ps PageSubscription) Remove(s *Subscriptions) {
	delete(s.ByChannelID[ps.ChannelID], ps.Alias)
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	delete(s.ByURLPageID[key], ps.ChannelID)
}

func (ps PageSubscription) Edit(s *Subscriptions) {
	ps.Remove(s)
	ps.Add(s)
}

func (ps PageSubscription) Name() string {
	return SubscriptionTypePage
}

func (ps PageSubscription) GetAlias() string {
	return ps.Alias
}

func (ps PageSubscription) GetFormattedSubscription() string {
	var events []string
	for _, event := range ps.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|", ps.Alias, ps.BaseURL, ps.PageID, strings.Join(events, ", "))
}

func (ps PageSubscription) IsValid() error {
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

func PageSubscriptionFromJSON(data io.Reader, subscriptionType string) (PageSubscription, error) {
	var ps PageSubscription
	err := json.NewDecoder(data).Decode(&ps)
	if err != nil {
		return ps, errors.New("error unmarshalling data")
	}

	if ps.PageID == "" {
		return ps, errors.New("pageID is required")
	}

	if subscriptionType != ps.Type {
		return ps, errors.New("subscription type mismatch")
	}

	return ps, nil
}

func (ps PageSubscription) ValidateSubscription(subs *Subscriptions) error {
	if err := ps.IsValid(); err != nil {
		return err
	}
	if channelSubscriptions, valid := subs.ByChannelID[ps.ChannelID]; valid {
		if _, ok := channelSubscriptions[ps.Alias]; ok {
			return errors.New(aliasAlreadyExist)
		}
	}
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	if urlPageIDSubscriptions, valid := subs.ByURLPageID[key]; valid {
		if _, ok := urlPageIDSubscriptions[ps.ChannelID]; ok {
			return errors.New(urlPageIDAlreadyExist)
		}
	}
	return nil
}
