package main

import "github.com/mattermost/mattermost-plugin-confluence/server/util/types"

// Client is the combined interface for all upstream APIs and convenience methods.
type Client interface {
	RESTService
}

// RESTService is the low-level interface for invoking the upstream service.
// Endpoint can be a "short" API URL path, including the version desired, like "v3/user",
// or a fully-qualified URL, with a non-empty scheme.
type RESTService interface {
	GetSelf() (*types.ConfluenceUser, error)
	GetSpaceData(string) (*SpaceResponse, error)
	GetPageData(int) (*PageResponse, error)
	GetSpaceKeyFromSpaceID(int64) (string, error)

	// location is "footer"/"inline" for Cloud comment events; DC ignores it.
	MentionAccountIDsInPage(pageID string) ([]string, error)
	MentionAccountIDsInComment(commentID, location string) ([]string, error)
}
