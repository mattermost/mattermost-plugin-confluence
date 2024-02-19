package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitArgs(t *testing.T) {
	for name, val := range map[string]struct {
		command        string
		expectedResult []string
		errMessage     string
	}{
		"command 1": {
			command:        "/confluence edit \"abc\"",
			expectedResult: []string{"/confluence", "edit", "abc"},
		},
		"command 2": {
			command:        "/confluence list     ",
			expectedResult: []string{"/confluence", "list"},
		},
		"command 3": {
			command:        "/confluence      subscribe",
			expectedResult: []string{"/confluence", "subscribe"},
		},
		"command 4": {
			command:        "/confluence edit \"  test     \"",
			expectedResult: []string{"/confluence", "edit", "test"},
		},
		"command 5": {
			command:        "/confluence subscribe",
			expectedResult: []string{"/confluence", "subscribe"},
		},
		"command 6": {
			command:        "/confluence unsubscribe \" test\"",
			expectedResult: []string{"/confluence", "unsubscribe", "test"},
		},
		"command 7": {
			command:    "/confluence edit \"abc  ",
			errMessage: "quotes not closed",
		},
	} {
		t.Run(name, func(t *testing.T) {
			args, err := SplitArgs(val.command)
			if err != nil {
				assert.Equal(t, val.errMessage, err.Error())
				return
			}
			assert.Equal(t, val.expectedResult, args)
		})
	}
}

func TestCreateConfluenceURL(t *testing.T) {
	for name, val := range map[string]struct {
		url            string
		expectedResult string
	}{
		"url": {
			url:            "http://test.com",
			expectedResult: "http://test.com",
		},
		"url https": {
			url:            "https://test.com",
			expectedResult: "https://test.com",
		},
		"url parameters": {
			url:            "https://test.com/test1/test2",
			expectedResult: "https://test.com",
		},
		"url query parameters": {
			url:            "https://test.com?test=test",
			expectedResult: "https://test.com",
		},
	} {
		t.Run(name, func(t *testing.T) {
			args, _ := CreateConfluenceURL(val.url)
			assert.Equal(t, val.expectedResult, args)
		})
	}
}

func TestIsConfluenceCloudURL(t *testing.T) {
	for name, val := range map[string]struct {
		url            string
		expectedResult bool
	}{
		"valid confluence cloud url": {
			url:            "https://test.atlassian.net/",
			expectedResult: true,
		},
		"Invalid confluence cloud url": {
			url:            "https://test.com",
			expectedResult: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := IsConfluenceCloudURL(val.url)
			assert.Equal(t, val.expectedResult, args)
		})
	}
}

func TestGetBodyForExcerpt(t *testing.T) {
	for name, val := range map[string]struct {
		url            string
		expectedResult string
	}{
		"html": {
			url:            "<html><head><title>Test</title></head></html>",
			expectedResult: "\nTest",
		},
		"html with styles": {
			url:            "<html><head><style>h1{color: maroon;margin-left: 40px;}</style></head><h1>Test</h1></html>",
			expectedResult: "\nTest",
		},
		"html with scripts": {
			url:            "<html><head><script>document.getElementById(test).innerHTML = Testing!;</script></head><h1>Test</h1></html>",
			expectedResult: "\nTest",
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := GetBodyForExcerpt(val.url)
			assert.Equal(t, val.expectedResult, args)
		})
	}
}
