package serializer

import (
	"fmt"
)

type ConfluenceConfig struct {
	ServerURL    string
	ClientID     string
	ClientSecret string
}

func (c ConfluenceConfig) GetFormattedConfig() string {
	return fmt.Sprintf("\n|%s|%s|%s|", c.ServerURL, c.ClientID, c.ClientSecret)
}

func FormattedConfigList(confluenceConfigs []*ConfluenceConfig) string {
	var configs, list string
	configHeader := "| Server URL | Client ID | Client Secret |\n| :----|:--------| :--------|"
	for _, config := range confluenceConfigs {
		configs += config.GetFormattedConfig()
	}

	if configs != "" {
		list = "#### Active Configurations \n" + configHeader + configs
	}

	return list
}
