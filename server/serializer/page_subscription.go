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

func (ps PageSubscription) Add(s *Subscriptions) error {
	s.EnsureDefaults()

	// Update OldAlias to current Alias as we don't need the old alias when creating a new subscription
	ps.OldAlias = ps.Alias

	if _, valid := s.ByChannelID[ps.ChannelID]; !valid {
		s.ByChannelID[ps.ChannelID] = make(StringSubscription)
	}
	if s.ByChannelID[ps.ChannelID] == nil {
		return errors.New("ByChannelID entry is nil")
	}
	s.ByChannelID[ps.ChannelID][ps.Alias] = ps
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)

	if _, ok := s.ByURLPageID[key]; !ok {
		s.ByURLPageID[key] = make(map[string][]string)
	}
	if s.ByURLPageID[key] == nil {
		return errors.New("ByURLPageID entry is nil")
	}
	s.ByURLPageID[key][ps.ChannelID] = ps.Events
	return nil
}

func (ps PageSubscription) Remove(s *Subscriptions) error {
	if s.ByChannelID == nil {
		return errors.New("ByChannelID map is nil")
	}
	if channelMap, ok := s.ByChannelID[ps.ChannelID]; ok {
		aliasToRemove := ps.OldAlias
		if aliasToRemove == "" {
			aliasToRemove = ps.Alias
		}

		if _, aliasOk := channelMap[aliasToRemove]; aliasOk {
			delete(channelMap, aliasToRemove)
		} else {
			return errors.New("alias not found in ByChannelID")
		}
	} else {
		return errors.New("channelID not found in ByChannelID")
	}
	key := store.GetURLPageIDCombinationKey(ps.BaseURL, ps.PageID)
	if s.ByURLPageID == nil {
		return errors.New("ByURLPageID map is nil")
	}
	if urlPageMap, ok := s.ByURLPageID[key]; ok {
		if _, channelOk := urlPageMap[ps.ChannelID]; channelOk {
			delete(urlPageMap, ps.ChannelID)
		} else {
			return errors.New("channelID not found in ByURLPageID entry")
		}
	} else {
		return errors.New("key not found in ByURLPageID")
	}
	return nil
}

func (ps PageSubscription) Edit(s *Subscriptions) error {
	if err := ps.Remove(s); err != nil {
		return err
	}
	if err := ps.Add(s); err != nil {
		return err
	}
	return nil
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
