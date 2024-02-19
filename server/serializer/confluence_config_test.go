package serializer

import (
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
)

func TestFormattedConfigList(t *testing.T) {
	tests := map[string]struct {
		config []*ConfluenceConfig
		result string
	}{
		"space subscription": {
			config: []*ConfluenceConfig{
				{
					ServerURL:    "test-ServerURL",
					ClientID:     "test-ClientID",
					ClientSecret: "test-ClientSecret",
				},
			},
			result: "#### Active Configurations \n| Server URL | Client ID | Client Secret |\n| :----|:--------| :--------|\n|test-ServerURL|test-ClientID|test-ClientSecret|",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer monkey.UnpatchAll()

			formattedList := FormattedConfigList(tc.config)
			assert.Equal(t, tc.result, formattedList)
		})
	}
}
