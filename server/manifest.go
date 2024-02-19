// Code generated by Mattermost. DO NOT EDIT.
package main

import (
	"encoding/json"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

var manifest *model.Manifest

const manifestStr = `
{
  "id": "com.mattermost.confluence",
  "name": "Confluence",
  "description": "Atlassian Confluence plugin for Mattermost.",
  "homepage_url": "https://github.com/mattermost/mattermost-plugin-confluence",
  "support_url": "https://github.com/mattermost/mattermost-plugin-confluence/issues",
  "release_notes_url": "https://github.com/mattermost/mattermost-plugin-confluence/releases/tag/v1.2.0",
  "icon_path": "assets/icon.svg",
  "version": "1.3.0",
  "min_server_version": "5.26.0",
  "server": {
    "executables": {
      "darwin-amd64": "server/dist/plugin-darwin-amd64",
      "linux-amd64": "server/dist/plugin-linux-amd64",
      "windows-amd64": "server/dist/plugin-windows-amd64.exe"
    },
    "executable": ""
  },
  "webapp": {
    "bundle_path": "webapp/dist/main.js"
  },
  "settings_schema": {
    "header": "",
    "footer": "",
    "settings": [
      {
        "key": "Secret",
        "display_name": "Webhook Secret:",
        "type": "generated",
        "help_text": "The secret used to authenticate the webhook to Mattermost.",
        "regenerate_help_text": "Regenerates the secret for the webhook URL endpoint. Regenerating the secret invalidates your existing Confluence integrations.",
        "placeholder": "",
        "default": null
      },
      {
        "key": "RolesAllowedToEditConfluenceSubscriptions",
        "display_name": "Mattermost Roles Allowed to Edit Confluence Subscriptions:",
        "type": "radio",
        "help_text": "Mattermost users who can subscribe channels to Confluence space or page.",
        "placeholder": "",
        "default": "system_admin",
        "options": [
          {
            "display_name": "All users",
            "value": "users"
          },
          {
            "display_name": "Users who can manage channel settings",
            "value": "channel_admin"
          },
          {
            "display_name": "Users who can manage teams",
            "value": "team_admin"
          },
          {
            "display_name": "System Admins",
            "value": "system_admin"
          }
        ]
      },
      {
        "key": "GroupsAllowedToEditConfluenceSubscriptions",
        "display_name": "Confluence Groups Allowed to Edit Confluence Subscriptions:",
        "type": "text",
        "help_text": "Comma-separated list of Group Names. List the Confluence user groups that can create subscriptions. If none are specified, any Confluence user can create a subscription.",
        "placeholder": "",
        "default": ""
      },
      {
        "key": "tokens",
        "display_name": "Confluence Config:",
        "type": "custom",
        "help_text": "",
        "placeholder": "",
        "default": []
      }
    ]
  }
}
`

func init() {
	json.NewDecoder(strings.NewReader(manifestStr)).Decode(&manifest)
}
