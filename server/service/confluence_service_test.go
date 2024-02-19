package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEndpointURL(t *testing.T) {
	for name, val := range map[string]struct {
		instanceURL    string
		path           string
		expectedResult string
	}{
		"get enpoint URL": {
			instanceURL:    "https://test.atlassian.net",
			path:           "/test/test1/test2",
			expectedResult: "https://test.atlassian.net/test/test1/test2",
		},
	} {
		t.Run(name, func(t *testing.T) {
			args, _ := GetEndpointURL(val.instanceURL, val.path)
			assert.Equal(t, val.expectedResult, args)
		})
	}
}
