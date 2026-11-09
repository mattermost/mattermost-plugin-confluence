package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestForgePollInterval(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override string
		want     time.Duration
	}{
		{name: "release build", override: "", want: 30 * time.Second},
		{name: "e2e override", override: "2", want: 2 * time.Second},
		{name: "not a number", override: "fast", want: 30 * time.Second},
		{name: "zero", override: "0", want: 30 * time.Second},
		{name: "negative", override: "-5", want: 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prev := forgePollIntervalSeconds
			t.Cleanup(func() { forgePollIntervalSeconds = prev })
			forgePollIntervalSeconds = tc.override

			assert.Equal(t, tc.want, forgePollInterval())
		})
	}
}

func TestAtlassianURLsOverridden(t *testing.T) {
	prevAuth, prevAPI := atlassianAuthBaseURL, atlassianAPIBaseURL
	t.Cleanup(func() { atlassianAuthBaseURL, atlassianAPIBaseURL = prevAuth, prevAPI })

	assert.False(t, atlassianURLsOverridden())

	atlassianAuthBaseURL = "http://e2e.atlassian.net:8092"
	assert.True(t, atlassianURLsOverridden())

	atlassianAuthBaseURL = defaultAtlassianAuthBaseURL
	atlassianAPIBaseURL = "http://e2e.atlassian.net:8092"
	assert.True(t, atlassianURLsOverridden())
}
