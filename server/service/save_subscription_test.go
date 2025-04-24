package service

import (
	"net/http"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

func TestSaveSpaceSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		newSubscription serializer.SpaceSubscription
		statusCode      int
		errorMessage    string
	}{
		"alias already exist": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: "TS",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "test",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: aliasAlreadyExist,
		},
		"url space key combination already exist": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: "TS",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: urlSpaceKeyAlreadyExist,
		},
		"subscription unique base url": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: "TS",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test1.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription unique space key": {
			newSubscription: serializer.SpaceSubscription{
				SpaceKey: "TS1",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
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
					"confluence_subs/test.com/TS": {
						"testtesttesttest": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
				ByURLPageID: map[string]serializer.StringArrayMap{
					"confluence_subs/test.com/1234": {
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
			statusCode, err := SaveSubscription(val.newSubscription)
			assert.Equal(t, val.statusCode, statusCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
			}
		})
	}
}

func TestSavePageSubscription(t *testing.T) {
	for name, val := range map[string]struct {
		newSubscription serializer.PageSubscription
		statusCode      int
		errorMessage    string
	}{
		"alias already exist": {
			newSubscription: serializer.PageSubscription{
				PageID: "1234",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "test",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: aliasAlreadyExist,
		},
		"url page id combination already exist": {
			newSubscription: serializer.PageSubscription{
				PageID: "1234",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttes1",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusBadRequest,
			errorMessage: urlPageIDAlreadyExist,
		},
		"subscription unique base url": {
			newSubscription: serializer.PageSubscription{
				PageID: "TS",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test1.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
		},
		"subscription unique space key": {
			newSubscription: serializer.PageSubscription{
				PageID: "12345",
				BaseSubscription: serializer.BaseSubscription{
					Alias:     "tes2",
					BaseURL:   "https://test.com",
					ChannelID: "testtesttesttest",
					Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				},
			},
			statusCode:   http.StatusOK,
			errorMessage: "",
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
					"confluence_subs/test.com/TS": {
						"testtesttesttest": {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
				ByURLPageID: map[string]serializer.StringArrayMap{
					"confluence_subs/test.com/1234": {
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
			errCode, err := SaveSubscription(val.newSubscription)
			assert.Equal(t, val.statusCode, errCode)
			if err != nil {
				assert.Equal(t, val.errorMessage, err.Error())
			}
		})
	}
}
