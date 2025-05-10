package service

import (
	"fmt"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

func TestDeleteSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		channelID string
		alias     string
		apiCalls  func(t *testing.T, channelID, alias string)
	}{
		"space subscription delete success": {
			channelID: "testtesttesttest",
			alias:     "test",
			apiCalls: func(t *testing.T, channelID, alias string) {
				err := DeleteSubscription(channelID, alias)
				assert.Nil(t, err)
			},
		},
		"page subscription delete success": {
			channelID: "testtesttesttes1",
			alias:     "test",
			apiCalls: func(t *testing.T, channelID, alias string) {
				err := DeleteSubscription(channelID, alias)
				assert.Nil(t, err)
			},
		},
		"subscription not found with alias": {
			channelID: "testtestesttest",
			alias:     "test1",
			apiCalls: func(t *testing.T, channelID, alias string) {
				err := DeleteSubscription(channelID, alias)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
		"no subscription for the channel": {
			channelID: "testtestesttesx",
			alias:     "test1",
			apiCalls: func(t *testing.T, channelID, alias string) {
				err := DeleteSubscription(channelID, alias)
				assert.NotNil(t, err)
				assert.Equal(t, fmt.Sprintf(subscriptionNotFound, alias), err.Error())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			subscriptions := serializer.Subscriptions{
				ByChannelID: map[string]serializer.StringSubscription{
					"testtesttesttest": {
						"test": serializer.SpaceSubscription{
							SpaceKey: "TS",
							BaseSubscription: serializer.BaseSubscription{
								Alias:     "test",
								BaseURL:   "https://test.com",
								ChannelID: "testtesttesttest",
								Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
							},
						},
					},
					"testtesttesttes1": {
						"test": serializer.PageSubscription{
							PageID: "1234",
							BaseSubscription: serializer.BaseSubscription{
								Alias:     "test",
								BaseURL:   "https://test.com",
								ChannelID: "testtesttesttest",
								Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
							},
						},
					},
				},
				ByURLSpaceKey: map[string]serializer.StringArrayMap{
					"testKey": {
						"testtesttesttest": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
				ByURLPageID: map[string]serializer.StringArrayMap{
					"testKey1": {
						"testtesttesttes1": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
			}
			monkey.Patch(GetSubscriptions, func() (serializer.Subscriptions, error) {
				return subscriptions, nil
			})
			monkey.Patch(store.AtomicModify, func(_ string, _ func(initialValue []byte) ([]byte, error)) error {
				return nil
			})
			val.apiCalls(t, val.channelID, val.alias)
		})
	}
}
