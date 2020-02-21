package serializer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	url2 "net/url"
	"strings"

	"github.com/Brightscout/mattermost-plugin-confluence/server/store"
)

type SpaceSubscription struct {
	SpaceKey string `json:"spaceKey"`
	BaseSubscription
}

func (ss SpaceSubscription) Add(s *Subscriptions) {
	if _, valid := s.ByChannelID[ss.ChannelID]; !valid {
		s.ByChannelID[ss.ChannelID] = make(StringSubscription)
	}
	s.ByChannelID[ss.ChannelID][ss.Alias] = ss
	key := store.GetURLSpaceKeyCombinationKey(ss.BaseURL, ss.SpaceKey)
	if _, ok := s.ByURLSpaceKey[key]; !ok {
		s.ByURLSpaceKey[key] = make(map[string][]string)
	}
	s.ByURLSpaceKey[key][ss.ChannelID] = ss.Events
}

func (ss SpaceSubscription) Remove(s *Subscriptions) {
	delete(s.ByChannelID[ss.ChannelID], ss.Alias)
	key := store.GetURLSpaceKeyCombinationKey(ss.BaseURL, ss.SpaceKey)
	delete(s.ByURLSpaceKey[key], ss.ChannelID)
}

func (ss SpaceSubscription) Edit(s *Subscriptions) {
	ss.Remove(s)
	ss.Add(s)
}

func (ss SpaceSubscription) Name() string {
	return SubscriptionTypeSpace
}

func (ss SpaceSubscription) GetFormattedSubscription() string {
	var events []string
	for _, event := range ss.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|", ss.Alias, ss.BaseURL, ss.SpaceKey, strings.Join(events, ", "))
}

func (ss SpaceSubscription) IsValid() error {
	// TODO : Clean subscription data
	if ss.Alias == "" {
		return errors.New("alias can not be empty")
	}
	if ss.BaseURL == "" {
		return errors.New("base url can not be empty")
	}
	if _, err := url2.Parse(ss.BaseURL); err != nil {
		return errors.New("enter a valid url")
	}
	if ss.SpaceKey == "" {
		return errors.New("space key can not be empty")
	}
	return nil
}

func SpaceSubscriptionFromJSON(data io.Reader) (SpaceSubscription, error) {
	var ps SpaceSubscription
	err := json.NewDecoder(data).Decode(&ps)
	return ps, err
}
