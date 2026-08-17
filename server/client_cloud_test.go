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
		_, _ = w.Write([]byte(`{"id":1,"key":"MM","name":"Mattermost"}`))
	}))
	defer ts.Close()

	client := newCloudClient(ts.URL, ts.Client())
	space, err := client.GetSpaceData("MM")

	require.NoError(t, err)
	assert.Equal(t, "MM", space.Key)
	assert.Equal(t, "/wiki/rest/api/space/MM?status=any", requestedPath)
}

func TestCloudClientGetSpaceDataForbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := newCloudClient(ts.URL, ts.Client()).GetSpaceData("MM")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestCloudClientGetPageData(t *testing.T) {
	var requestedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"12345","title":"Release notes"}`))
	}))
	defer ts.Close()

	page, err := newCloudClient(ts.URL, ts.Client()).GetPageData(12345)

	require.NoError(t, err)
	assert.Equal(t, "Release notes", page.Title)
	assert.Equal(t, "/wiki/rest/api/content/12345", requestedPath)
}
