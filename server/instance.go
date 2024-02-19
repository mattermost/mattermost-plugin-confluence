package main

import (
	"fmt"

	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type InstanceType string

const (
	CloudInstanceType     = InstanceType("cloud")
	ServerInstanceType    = InstanceType("server")
	CloudInstance         = "cloud"
	ServerInstance        = "server"
	AdminMattermostUserID = "admin"
)

type Instance interface {
	GetClient(connection *Connection) (Client, error)
	GetManageAppsURL() string
	GetManageWebhooksURL() string
	GetURL() string
	GetOAuth2Config(bool) (*oauth2.Config, error)

	Common() *InstanceCommon
	types.Value
}

// InstanceCommon contains metadata common for both cloud and server instances.
// The fields lack `json` modifiers to be backwards compatible with v2.
type InstanceCommon struct {
	*Plugin       `json:"-"`
	PluginVersion string `json:",omitempty"`

	InstanceID types.ID
	Alias      string
	Type       InstanceType
	IsV2Legacy bool

	SetupWizardUserID string
}

func newInstanceCommon(p *Plugin, instanceType InstanceType, instanceID types.ID) *InstanceCommon {
	return &InstanceCommon{
		Plugin:        p,
		Type:          instanceType,
		InstanceID:    instanceID,
		PluginVersion: manifest.Version,
	}
}

func (ic *InstanceCommon) AsConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"type":        string(ic.Type),
		"instance_id": string(ic.InstanceID),
		"alias":       ic.Alias,
	}
}

func (ic *InstanceCommon) GetID() types.ID {
	return ic.InstanceID
}

func (ic *InstanceCommon) Common() *InstanceCommon {
	return ic
}

func (ic *InstanceCommon) GetRedirectURL() string {
	return fmt.Sprintf("%s%s", ic.GetPluginURL(), instancePath(routeUserComplete, ic.InstanceID))
}
