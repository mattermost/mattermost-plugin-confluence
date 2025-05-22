package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

type PluginAPI interface {
	GetBundlePath() (string, error)
}

type HandlerFunc func(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse

type Handler struct {
	handlers       map[string]HandlerFunc
	defaultHandler HandlerFunc
}

const (
	specifyAlias              = "Please specify a subscription name."
	subscriptionDeleteSuccess = "Subscription **%s** has been deleted."
	noChannelSubscription     = "No subscriptions found for this channel."
	commonHelpText            = "###### Mattermost Confluence Plugin - Slash Command Help\n\n" +
		"* `/confluence connect` - Connect your Mattermost user to Confluence.\n" +
		"* `/confluence disconnect` - Disconnect your Mattermost user from Confluence.\n" +
		"* `/confluence subscribe` - Subscribe the current channel to notifications from Confluence.\n" +
		"* `/confluence unsubscribe \"<name>\"` - Unsubscribe the current channel from notifications associated with the given subscription name.\n" +
		"* `/confluence list` - List all subscriptions for the current channel.\n" +
		"* `/confluence edit \"<name>\"` - Edit the subscription settings associated with the given subscription name.\n"

	sysAdminHelpText = "\n###### For System Administrators:\n" +
		"Setup Instructions:\n" +
		"* `/confluence install cloud` - Connect Mattermost to a Confluence Cloud instance.\n" +
		"* `/confluence install server` - Connect Mattermost to a Confluence Server or Data Center instance.\n"

	invalidCommand              = "Invalid command."
	installOnlySystemAdmin      = "`/confluence install` can only be run by a system administrator."
	commandsOnlySystemAdmin     = "`/confluence` commands can only be run by a system administrator."
	errorUserLacksChannelAccess = "Cannot perform operation: user does not have access to the channel."
	disconnectedUser            = "User not connected. Please use `/confluence connect`."
	errorExecutingCommand       = "Error executing the command, please retry."
	oauth2ConnectPath           = "%s/oauth2/connect"
)

const (
	installCloudHelp = `
To finish the configuration, add a new app in your Confluence Cloud instance following these steps:
1. Navigate to **Settings > Apps > Manage Apps**.
2. Choose **Settings** at the bottom of the page, enable development mode, and apply the change. Development mode allows you to install apps from outside of the Atlassian Marketplace.
3. Press **Upload App**.
4. In **From this URL**, enter: %s
5. Once installed, you will see the "Installed and ready to go!" message.
`
)

var ConfluenceCommandHandler = Handler{
	handlers: map[string]HandlerFunc{
		"list":           listChannelSubscription,
		"unsubscribe":    deleteSubscription,
		"install/cloud":  showInstallCloudHelp,
		"install/server": showInstallServerHelp,
		"connect":        executeConnect,
		"disconnect":     executeDisconnect,
		"help":           confluenceHelpCommand,
	},
	defaultHandler: executeConfluenceDefault,
}

func GetCommand(pAPI PluginAPI) (*model.Command, error) {
	iconData, err := command.GetIconData(pAPI, "assets/icon.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	return &model.Command{
		Trigger:              "confluence",
		DisplayName:          "Confluence",
		Description:          "Integration with Confluence.",
		AutoComplete:         true,
		AutoCompleteDesc:     "Available commands: subscribe, list, unsubscribe, edit, install, help.",
		AutoCompleteHint:     "[command]",
		AutocompleteData:     getAutoCompleteData(),
		AutocompleteIconData: iconData,
	}, nil
}

func getAutoCompleteData() *model.AutocompleteData {
	confluence := model.NewAutocompleteData("confluence", "[command]", "Available commands: subscribe, list, unsubscribe, edit, install, help")

	install := model.NewAutocompleteData("install", "", "Connect Mattermost to a Confluence instance")
	installItems := []model.AutocompleteListItem{{
		HelpText: "Connect Mattermost to a Confluence Cloud instance",
		Item:     "cloud",
	}, {
		HelpText: "Connect Mattermost to a Confluence Server or Data Center instance",
		Item:     "server",
	}}
	install.AddStaticListArgument("", false, installItems)
	confluence.AddCommand(install)

	list := model.NewAutocompleteData("list", "", "List all subscriptions for the current channel")
	confluence.AddCommand(list)

	edit := model.NewAutocompleteData("edit", "[name]", "Edit the subscription settings associated with the given subscription name")
	edit.AddDynamicListArgument("name", "api/v1/autocomplete/GetChannelSubscriptions", false)
	confluence.AddCommand(edit)

	subscribe := model.NewAutocompleteData("subscribe", "", "Subscribe the current channel to notifications from Confluence")
	confluence.AddCommand(subscribe)

	unsubscribe := model.NewAutocompleteData("unsubscribe", "[name]", "Unsubscribe the current channel from notifications associated with the given subscription name")
	unsubscribe.AddDynamicListArgument("name", "api/v1/autocomplete/GetChannelSubscriptions", false)
	confluence.AddCommand(unsubscribe)

	help := model.NewAutocompleteData("help", "", "Show confluence slash command help")
	confluence.AddCommand(help)

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your Confluence account")
	confluence.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your Confluence account")
	confluence.AddCommand(disconnect)

	return confluence
}

func executeConfluenceDefault(_ *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	out := invalidCommand + "\n\n"
	out += getFullHelpText(context, args...)

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         out,
	}
}

func postCommandResponse(context *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    config.BotUserID,
		ChannelId: context.ChannelId,
		Message:   text,
	}
	_ = config.Mattermost.SendEphemeralPost(context.UserId, post)
}

func (ch Handler) Handle(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if len(args) == 0 {
		return ch.handlers["help"](p, context, "")
	}
	for n := len(args); n > 0; n-- {
		h := ch.handlers[strings.Join(args[:n], "/")]
		if h != nil {
			return h(p, context, args[n:]...)
		}
	}
	return ch.defaultHandler(p, context, args...)
}

func (p *Plugin) responsef(commandArgs *model.CommandArgs, format string, args ...interface{}) *model.CommandResponse {
	postCommandResponse(commandArgs, fmt.Sprintf(format, args...))
	return &model.CommandResponse{}
}

func executeConnect(p *Plugin, context *model.CommandArgs, _ ...string) *model.CommandResponse {
	isAdmin := util.IsSystemAdmin(context.UserId)

	pluginConfig := config.GetConfig()
	if pluginConfig.ConfluenceURL == "" || !pluginConfig.IsOAuthConfigured() {
		if isAdmin {
			return p.responsef(context, "OAuth config not set for Confluence plugin. Please run `/confluence install server`")
		}
		return p.responsef(context, "OAuth config not set for Confluence plugin. Please ask the admin to setup OAuth for the plugin")
	}
	confluenceURL := pluginConfig.GetConfluenceBaseURL()
	confluenceURL = strings.TrimSuffix(confluenceURL, "/")

	conn, err := store.LoadConnection(confluenceURL, context.UserId)
	if err == nil && len(conn.ConfluenceAccountID()) != 0 {
		return p.responsef(context,
			"You already have a Confluence account linked to your Mattermost account. Please use `/confluence disconnect` to disconnect.")
	} else if err != nil {
		p.client.Log.Error("Error loading connection for the user", "UserID", context.UserId, "error", err.Error())
		return p.responsef(context, "Failed to complete the **connection** request. Error: %v", err)
	}

	link := fmt.Sprintf(oauth2ConnectPath, util.GetPluginURL())
	return p.responsef(context, "[Click here to link your Confluence account](%s)", link)
}

func executeDisconnect(p *Plugin, commArgs *model.CommandArgs, _ ...string) *model.CommandResponse {
	user, err := store.LoadUser(commArgs.UserId)
	if err != nil {
		p.client.Log.Error("Error loading  the user from store", "UserID", commArgs.UserId, "error", err.Error())
		return p.responsef(commArgs, "Failed to complete the **disconnection** request. Error: %v", err)
	}
	confluenceURL := user.InstanceURL

	disconnected, err := p.DisconnectUser(confluenceURL, commArgs.UserId)
	if err != nil {
		if errors.Cause(err) == store.ErrNotFound {
			return p.responsef(commArgs, "Your account is not connected to Confluence.")
		}

		p.client.Log.Error("Error disconnecting the user", "UserID", commArgs.UserId, "error", err.Error())
		return p.responsef(commArgs, "Failed to complete the **disconnection** request. Error: %v", err)
	}
	return p.responsef(commArgs, "You have successfully disconnected your Confluence account (**%s**).", disconnected.DisplayName)
}

func showInstallCloudHelp(_ *Plugin, context *model.CommandArgs, _ ...string) *model.CommandResponse {
	if !util.IsSystemAdmin(context.UserId) {
		postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	cloudURL := util.GetPluginURL() + util.GetAtlassianConnectURLPath()
	postCommandResponse(context, fmt.Sprintf(installCloudHelp, cloudURL))
	return &model.CommandResponse{}
}

func showInstallServerHelp(p *Plugin, context *model.CommandArgs, _ ...string) *model.CommandResponse {
	if !util.IsSystemAdmin(context.UserId) {
		postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	err := p.flowManager.StartSetupWizard(context.UserId, "")
	if err != nil {
		p.client.Log.Error("Failed to start setup wizard", "user_id", context.UserId, "error", err.Error())
		return &model.CommandResponse{
			Text: "Failed to start setup wizard",
		}
	}

	return &model.CommandResponse{
		Text: "Please continue with confluence bot DM for the setup.",
	}
}

func deleteSubscription(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	userID := context.UserId
	channelID := context.ChannelId

	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		conn, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				postCommandResponse(context, disconnectedUser)
				return &model.CommandResponse{}
			}

			p.client.Log.Error("Error loading the connection for the user", "UserID", context.UserId, "error", err.Error())
			postCommandResponse(context, errorExecutingCommand)
			return &model.CommandResponse{}
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			postCommandResponse(context, disconnectedUser)
			return &model.CommandResponse{}
		}
	} else if !util.IsSystemAdmin(userID) {
		postCommandResponse(context, commandsOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	if len(args) == 0 {
		postCommandResponse(context, specifyAlias)
		return &model.CommandResponse{}
	}

	if !p.hasChannelAccess(userID, channelID) {
		postCommandResponse(context, errorUserLacksChannelAccess)
		return &model.CommandResponse{}
	}

	alias := strings.Join(args, " ")
	if err := service.DeleteSubscription(channelID, alias); err != nil {
		p.client.Log.Error("Error deleting the subscription", "subscription alias", alias, "error", err.Error())
		postCommandResponse(context, err.Error())
		return &model.CommandResponse{}
	}

	postCommandResponse(context, fmt.Sprintf(subscriptionDeleteSuccess, alias))
	return &model.CommandResponse{}
}

func listChannelSubscription(p *Plugin, context *model.CommandArgs, _ ...string) *model.CommandResponse {
	pluginConfig := config.GetConfig()
	if pluginConfig.ServerVersionGreaterthan9 {
		conn, err := store.LoadConnection(pluginConfig.ConfluenceURL, context.UserId)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				postCommandResponse(context, disconnectedUser)
				return &model.CommandResponse{}
			}

			p.client.Log.Error("Error loading the connection for the user", "UserID", context.UserId, "error", err.Error())
			postCommandResponse(context, errorExecutingCommand)
			return &model.CommandResponse{}
		}

		if len(conn.ConfluenceAccountID()) == 0 {
			postCommandResponse(context, disconnectedUser)
			return &model.CommandResponse{}
		}
	} else if !util.IsSystemAdmin(context.UserId) {
		postCommandResponse(context, commandsOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	channelSubscriptions, gErr := service.GetSubscriptionsByChannelID(context.ChannelId)
	if gErr != nil {
		p.client.Log.Error("Error getting the subscription for the channel", "ChannelID", context.ChannelId, "UserID", context.UserId, "error", gErr.Error())
		postCommandResponse(context, gErr.Error())
		return &model.CommandResponse{}
	}

	if len(channelSubscriptions) == 0 {
		postCommandResponse(context, noChannelSubscription)
		return &model.CommandResponse{}
	}
	list := serializer.FormattedSubscriptionList(channelSubscriptions)
	postCommandResponse(context, list)
	return &model.CommandResponse{}
}

func confluenceHelpCommand(_ *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	pluginConfig := config.GetConfig()
	if !pluginConfig.ServerVersionGreaterthan9 && !util.IsSystemAdmin(context.UserId) {
		postCommandResponse(context, commandsOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	helpText := getFullHelpText(context, args...)

	postCommandResponse(context, helpText)
	return &model.CommandResponse{}
}

func getFullHelpText(context *model.CommandArgs, _ ...string) string {
	helpText := commonHelpText
	if util.IsSystemAdmin(context.UserId) {
		helpText += sysAdminHelpText
	}
	return helpText
}
