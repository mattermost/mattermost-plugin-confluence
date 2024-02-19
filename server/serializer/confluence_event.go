package serializer

import (
	"github.com/mattermost/mattermost/server/public/model"
)

type ConfluenceEvent interface {
	GetNotificationPost(string, string, string) *model.Post
	GetPageID() string
	GetSpaceKey() string
	GetURL() string
}
