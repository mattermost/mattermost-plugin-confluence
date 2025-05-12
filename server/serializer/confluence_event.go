package serializer

import (
	"github.com/mattermost/mattermost/server/public/model"
)

type ConfluenceEvent interface {
	GetNotificationPost(string) *model.Post
	GetURL() string
	GetSpaceKey() string
	GetPageID() string
}

// for handling of Confluence server version greater than 9 notifications
type ConfluenceEventV2 interface {
	GetNotificationPost(string, string, string) *model.Post
	GetURL() string
	GetSpaceKey() string
	GetPageID() string
}
