package main

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

type APIError struct {
	StatusCode int
	Path       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("confluence request for %s returned %d", e.Path, e.StatusCode)
}

func (e *APIError) IsAccessDenied() bool {
	switch e.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	default:
		return false
	}
}

func withAPIStatus(err error, path string, statusCode int) error {
	if err == nil || statusCode == 0 {
		return err
	}
	return errors.Wrap(&APIError{StatusCode: statusCode, Path: path}, err.Error())
}

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
