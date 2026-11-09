package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudClientGetSpaceData(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"spaceKey":"MM","routeOverrideEnabled":false,"_links":{}}`))
	}))
	defer ts.Close()

	client := newCloudClient(ts.URL, ts.Client())
	space, err := client.GetSpaceData("MM")

	require.NoError(t, err)
	assert.Equal(t, "MM", space.Key)
	assert.Equal(t, "/wiki/rest/api/space/MM/settings", requestedPath)
}

func TestCloudClientGetSpaceDataEscapesPersonalSpaceKey(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"routeOverrideEnabled":false,"_links":{}}`))
	}))
	defer ts.Close()

	space, err := newCloudClient(ts.URL, ts.Client()).GetSpaceData("~5b08914c")

	require.NoError(t, err)
	assert.Equal(t, "~5b08914c", space.Key)
	assert.Equal(t, "/wiki/rest/api/space/~5b08914c/settings", requestedPath)
}

func TestCloudClientGetSpaceDataErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusGone} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))

		_, err := newCloudClient(ts.URL, ts.Client()).GetSpaceData("MM")
		ts.Close()

		var apiErr *APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, status, apiErr.StatusCode)
	}
}

func TestCloudClientGetPageData(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"start":0,"limit":100,"size":0,"restrictionsHash":"h","_links":{}}`))
	}))
	defer ts.Close()

	page, err := newCloudClient(ts.URL, ts.Client()).GetPageData(12345)

	require.NoError(t, err)
	assert.Equal(t, "12345", page.ID)
	assert.Equal(t, "/wiki/rest/api/content/12345/restriction", requestedPath)
}

func TestCloudClientGetPageDataNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := newCloudClient(ts.URL, ts.Client()).GetPageData(12345)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.True(t, apiErr.IsAccessDenied())
}
