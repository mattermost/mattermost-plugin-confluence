package service

import (
	"fmt"
	"net/http"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

func TestGetChannelSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		channelID    string
		alias        string
		statusCode   int
		errorMessage string
	}{
		"get subscription success": {
			channelID:    "testChannelID",
			alias:        "test",
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription not found for alias": {
			channelID:    "testChannelID",
			alias:        "test4",
			statusCode:   http.StatusBadRequest,
			errorMessage: fmt.Sprintf(subscriptionNotFound, "test4"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()
			subscriptions := serializer.Subscriptions{
				ByChannelID: map[string]serializer.StringSubscription{
					"testChannelID": {
						"test": &serializer.SpaceSubscription{
							SpaceKey: "TS",
							BaseSubscription: serializer.BaseSubscription{
								Alias:     "test",
								BaseURL:   "https://test.com",
								ChannelID: "testChannelID",
								Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
							},
						},
					},
					"testChannelID3": {
						"test": &serializer.PageSubscription{
							PageID: "1234",
							BaseSubscription: serializer.BaseSubscription{
								Alias:     "test",
								BaseURL:   "https://test.com",
								ChannelID: "testChannelID",
								Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
							},
						},
					},
				},
				ByURLSpaceKey: map[string]serializer.StringStringArrayMap{
					"confluence_subscription/test.com/TS": {
						"testChannelID": {
							"testUserID": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
						},
					},
				},
				ByURLPageID: map[string]serializer.StringStringArrayMap{
					"confluence_subscription/test.com/1234": {
						"testChannelID3": {
							"testUserID": {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
						},
					},
				},
			}
			monkey.Patch(GetSubscriptions, func() (serializer.Subscriptions, error) {
				return subscriptions, nil
			})
			subscription, errCode, err := GetChannelSubscription(val.channelID, val.alias)
			assert.Equal(t, val.statusCode, errCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
				return
			}
			assert.NotNil(t, subscription)
			assert.Equal(t, subscription.GetAlias(), val.alias)
		})
	}
}
