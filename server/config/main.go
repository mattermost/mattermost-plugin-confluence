package config

import (
	"encoding/json"
	"strings"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

const (
	HeaderMattermostUserID = "Mattermost-User-Id"
)

var (
	config     atomic.Value
	Mattermost plugin.API
	BotUserID  string
)

type Configuration struct {
	Secret                      string `json:"secret"`
	EncryptionKey               string `json:"encryptionKey"` // The encryption key used to encrypt tokens
	AdminAPIToken               string `json:"adminAPIToken"` // API token from Confluence Data Center
	ConfluenceOAuthClientID     string
	ConfluenceOAuthClientSecret string
	ConfluenceURL               string
	ServerVersionGreaterthan9   bool
}

func GetConfig() *Configuration {
	return config.Load().(*Configuration)
}

func SetConfig(c *Configuration) {
	config.Store(c)
}

func (c *Configuration) ProcessConfiguration() error {
	c.Secret = strings.TrimSpace(c.Secret)

	return nil
}

func (c *Configuration) IsValid() error {
	if len(c.Secret) != 32 {
		return errors.New("please provide a valid 32-character Webhook Secret")
	}

	if len(c.EncryptionKey) != 32 {
		return errors.New("please provide a valid 32-character Encryption Key")
	}

	if c.AdminAPIToken != "" && len(c.AdminAPIToken) < 32 {
		return errors.New("please provide a valid Admin API Token with at least 32 characters")
	}

	return nil
}

func (c *Configuration) Sanitize() {
	// Ensure ConfluenceURL does not have trailing slashes by trimming any '/'
	c.ConfluenceURL = strings.TrimRight(c.ConfluenceURL, "/")

	c.ConfluenceOAuthClientID = strings.TrimSpace(c.ConfluenceOAuthClientID)
	c.ConfluenceOAuthClientSecret = strings.TrimSpace(c.ConfluenceOAuthClientSecret)
}

func (c *Configuration) IsOAuthConfigured() bool {
	return (c.ConfluenceOAuthClientID != "" && c.ConfluenceOAuthClientSecret != "")
}

func (c *Configuration) ToMap() (map[string]interface{}, error) {
	var out map[string]interface{}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Configuration) GetConfluenceBaseURL() string {
	return c.ConfluenceURL
}
