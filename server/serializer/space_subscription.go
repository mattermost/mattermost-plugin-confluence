package serializer

import (
	"encoding/json"
	"errors"
	"fmt"
	url2 "net/url"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

type SpaceSubscription struct {
	SpaceKey string `json:"spaceKey"`
	BaseSubscription
	SpaceID string
	UserID  string
}

type OldSubscriptionForSpace struct {
	SpaceSubscription
}

type OldSpaceSubscription struct {
	OldSubscription OldSubscriptionForSpace `json:"oldSubscription"`
}

func (ss *SpaceSubscription) Add(s *Subscriptions) {
	if _, valid := s.ByChannelID[ss.ChannelID]; !valid {
		s.ByChannelID[ss.ChannelID] = make(StringSubscription)
	}
	s.ByChannelID[ss.ChannelID][ss.Alias] = ss
	key := store.GetURLSpaceKeyCombinationKey(ss.BaseURL, ss.SpaceKey)

	if _, ok := s.ByURLSpaceKey[key]; !ok {
		s.ByURLSpaceKey[key] = make(map[string]StringArrayMap)
	}
	if _, ok := s.ByURLSpaceKey[key][ss.ChannelID]; !ok {
		s.ByURLSpaceKey[key][ss.ChannelID] = make(map[string][]string)
	}

	s.ByURLSpaceKey[key][ss.ChannelID][ss.UserID] = ss.Events

	if s.BySpaceID == nil {
		s.BySpaceID = make(map[string]string)
	}
	s.BySpaceID[ss.SpaceID] = ss.SpaceKey
}

func (ss *SpaceSubscription) Remove(s *Subscriptions) {
	delete(s.ByChannelID[ss.ChannelID], ss.Alias)
	key := store.GetURLSpaceKeyCombinationKey(ss.BaseURL, ss.SpaceKey)
	delete(s.ByURLSpaceKey[key], ss.ChannelID)
}

func (ss *SpaceSubscription) Edit(s *Subscriptions) {
	ss.Remove(s)
	ss.Add(s)
}

func (ss *SpaceSubscription) Name() string {
	return SubscriptionTypeSpace
}

func (ss *SpaceSubscription) GetAlias() string {
	return ss.Alias
}

func (ss *SpaceSubscription) GetConfluenceURL() string {
	return ss.GetSubscription().BaseURL
}

func (ss *SpaceSubscription) GetUserID() string {
	return ss.UserID
}

func (ss *SpaceSubscription) GetEvents() []string {
	return ss.Events
}

func (ss *SpaceSubscription) GetSpaceKeyOrPageID() string {
	return ss.SpaceKey
}

func (ss *SpaceSubscription) GetFormattedSubscription() string {
	var events []string
	for _, event := range ss.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|", ss.Alias, ss.BaseURL, ss.SpaceKey, strings.Join(events, ", "))
}

func (ss *SpaceSubscription) GetOldFormattedSubscription() string {
	var events []string
	for _, event := range ss.Events {
		events = append(events, eventDisplayName[event])
	}
	return fmt.Sprintf("\n|%s|%s|%s|%s|%s|", ss.Alias, ss.BaseURL, ss.SpaceKey, ss.ChannelID, strings.Join(events, ", "))
}

func (ss *SpaceSubscription) IsValid() error {
	if ss.Alias == "" {
		return errors.New("subscription name can not be empty")
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
	if ss.ChannelID == "" {
		return errors.New("channel id can not be empty")
	}
	return nil
}
func SpaceSubscriptionFromJSON(body []byte) (*SpaceSubscription, error) {
	var ps SpaceSubscription
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, err
	}
	return &ps, nil
}

func OldSpaceSubscriptionFromJSON(body []byte) (*SpaceSubscription, error) {
	var ps OldSpaceSubscription
	if err := json.Unmarshal(body, &ps); err != nil {
		return nil, err
	}
	return &ps.OldSubscription.SpaceSubscription, nil
}

func (ss *SpaceSubscription) UpdateSpaceIDAndUserID(spaceID, userID string) *SpaceSubscription {
	ss.SpaceID = spaceID
	ss.UserID = userID
	return ss
}

func (ss *SpaceSubscription) UpdateUserID(userID string) *SpaceSubscription {
	ss.UserID = userID
	return ss
}

func (ss *SpaceSubscription) GetChannelID() string {
	return ss.BaseSubscription.ChannelID
}

func (ss *SpaceSubscription) GetSubscription() *SpaceSubscription {
	return &SpaceSubscription{
		SpaceKey:         ss.SpaceKey,
		BaseSubscription: ss.BaseSubscription,
		SpaceID:          ss.SpaceID,
		UserID:           ss.UserID,
	}
}

func (ss *SpaceSubscription) ValidateSubscription(subs *Subscriptions) error {
	if err := ss.IsValid(); err != nil {
		return err
	}
	if channelSubscriptions, valid := subs.ByChannelID[ss.ChannelID]; valid {
		if _, ok := channelSubscriptions[ss.Alias]; ok {
			return errors.New(AliasAlreadyExist)
		}
	}
	key := store.GetURLSpaceKeyCombinationKey(ss.BaseURL, ss.SpaceKey)
	if urlSpaceKeySubscriptions, valid := subs.ByURLSpaceKey[key]; valid {
		if _, ok := urlSpaceKeySubscriptions[ss.ChannelID]; ok {
			return errors.New(URLSpaceKeyAlreadyExist)
		}
	}
	return nil
}
