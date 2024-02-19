package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	storePackage "github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/kvstore"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type HandlerFunc func(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse

type Handler struct {
	handlers       map[string]HandlerFunc
	defaultHandler HandlerFunc
}

const commandTrigger = "confluence"

const helpTextHeader = "###### Mattermost Confluence Plugin - Slash Command Help\n"

const (
	specifyAlias              = "Please specify a subscription name."
	subscriptionDeleteSuccess = "Subscription **%s** has been deleted."
	noChannelSubscription     = "No subscriptions found for this channel."
	noSavedConfig             = "No saved config found"
	commonHelpText            = "###### Mattermost Confluence Plugin - Slash Command Help\n\n" +
		"* `/confluence connect [confluenceURL]` - Connect your Mattermost account to your Confluence account\n" +
		"* `/confluence disconnect [confluenceURL]` - Disconnect your Mattermost account from your Confluence account\n" +
		"* `/confluence subscribe` - Subscribe the current channel to notifications from Confluence.\n" +
		"* `/confluence unsubscribe \"<name>\"` - Unsubscribe the current channel from notifications associated with the given subscription name.\n" +
		"* `/confluence list` - List all subscriptions for the current channel.\n" +
		"* `/confluence edit \"<name>\"` - Edit the subscription settings associated with the given subscription name.\n"

	sysAdminHelpText = "\n###### For System Administrators:\n" +
		"Setup Instructions:\n" +
		"* `/confluence install cloud [confluenceURL]` - Connect Mattermost to a Confluence Cloud instance.\n" +
		"* `/confluence install server [confluenceURL]` - Connect Mattermost to a Confluence Server or Data Center instance.\n" +
		"Uninstall Confluence instances:\n" +
		"* `/confluence uninstall cloud [confluenceURL]` - Disconnect Mattermost from a Confluence Cloud instance located at <confluenceURL>\n" +
		"* `/confluence uninstall server [confluenceURL]` - Disconnect Mattermost from a Confluence Server or Data Center instance located at <confluenceURL>\n" +
		"Manage Confluence instance Configurations:\n" +
		"* `/confluence config add [confluenceURL]` - Add config for the confluence instance <confluenceURL>.\n" +
		"* `/confluence config list` - List all the added configs.\n" +
		"* `/confluence config delete \"<name>\"` - Delete config for the confluence instance.\n"

	migrationCommandsHelpText = "Manage Confluence subscription migrations:\n" +
		"* `/confluence migrate list` - List all the old subscriptions to be migrated.\n" +
		"* `/confluence migrate start` - Start the migration of old subscriptions.\n" +
		"* `/confluence migrate cleanup` - Delete all the old subscriptions.\n"

	invalidCommand              = "Invalid command."
	installOnlySystemAdmin      = "`/confluence install` can only be run by a system administrator."
	configNotFoundError         = "configuration not found for %s. Please ask system admin to add config for %s in plugin configuration"
	configServerURL             = "Server URL"
	configClientID              = "Client ID"
	configClientSecret          = "Client Secret"
	configAPIEndpoint           = "%s/api/v4/actions/dialogs/open"
	configModalTitle            = "Confluence Config"
	configPerPage               = 10
	NoOldSubscriptionsMsg       = "No old subscriptions found for migration"
	NoOldSubscriptionsDeleteMsg = "No old subscriptions were found for cleanup."
	MigrationCompletedMsg       = "The migration process has been completed. Please refer to the server logs for more information."
	CleanupCompletedMsg         = "The cleanup process has been completed. Please refer to the server logs for more information."
	CleanupWaitMsg              = "Your cleanup request is being processed. Please wait." // #nosec G101
	MigrationWaitMsg            = "Your migration request is being processed. Please wait."
	configDialogueEndpoint      = "%s/config/%s/%s"
)

var ConfluenceCommandHandler = Handler{
	handlers: map[string]HandlerFunc{
		"connect":         executeConnect,
		"disconnect":      executeDisconnect,
		"list":            listChannelSubscription,
		"unsubscribe":     deleteSubscription,
		"install/cloud":   showInstallCloudHelp,
		"install/server":  showInstallServerHelp,
		"uninstall":       executeInstanceUninstall,
		"help":            confluenceHelpCommand,
		"config/add":      addConfig,
		"config/list":     listConfig,
		"config/delete":   deleteConfig,
		"migrate/list":    listOldSubscriptions,
		"migrate/start":   startSubscriptionMigration,
		"migrate/cleanup": deleteOldSubscriptions,
	},
	defaultHandler: executeConfluenceDefault,
}

func (p *Plugin) registerConfluenceCommand() error {
	// Optimistically unregister what was registered before
	_ = p.API.UnregisterCommand("", commandTrigger)

	command, err := p.GetCommand()
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	err = p.API.RegisterCommand(command)
	if err != nil {
		return errors.Wrapf(err, "failed to register /%s command", commandTrigger)
	}

	return nil
}

func (p *Plugin) GetCommand() (*model.Command, error) {
	iconData, err := command.GetIconData(p.API, "assets/icon.svg")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get icon data")
	}

	command := &model.Command{
		Trigger:              "confluence",
		DisplayName:          "Confluence",
		Description:          "Integration with Confluence.",
		AutoComplete:         true,
		AutoCompleteDesc:     "Available commands: subscribe, config, list, unsubscribe, edit, install, help.",
		AutoCompleteHint:     "[command]",
		AutocompleteData:     getAutoCompleteData(false),
		AutocompleteIconData: iconData,
	}

	oldSubscriptions, getErr := service.GetOldSubscriptions()
	if getErr != nil {
		return nil, getErr
	}

	if len(oldSubscriptions) != 0 {
		command.AutoCompleteDesc = "Available commands: subscribe, config, migrate, list, unsubscribe, edit, install, help."
		command.AutocompleteData = getAutoCompleteData(true)
	}

	return command, nil
}

func getAutoCompleteData(showMigrateCommands bool) *model.AutocompleteData {
	confluence := model.NewAutocompleteData("confluence", "[command]", "Available commands: subscribe, config, list, unsubscribe, edit, install, help")

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

	uninstall := model.NewAutocompleteData("uninstall", "", "Connect Mattermost to a Confluence instance")
	uninstallItems := []model.AutocompleteListItem{{
		HelpText: "Disconnect Mattermost to a Confluence Cloud instance",
		Item:     "cloud",
	}, {
		HelpText: "Disconnect Mattermost to a Confluence Server or Data Center instance",
		Item:     "server",
	}}
	uninstall.AddStaticListArgument("", false, uninstallItems)
	confluence.AddCommand(uninstall)

	config := model.NewAutocompleteData("config", "", "Config related options for confluence instances")

	addConfig := model.NewAutocompleteData("add", "[instance]", "Add config for the confluence instance")
	addConfig.AddDynamicListArgument("instance", "api/v1/autocomplete/installed-instances", false)

	listConfig := model.NewAutocompleteData("list", "", "List all the added configs")

	deleteConfig := model.NewAutocompleteData("delete", "[instance]", "Delete config for the confluence instance")
	deleteConfig.AddDynamicListArgument("instance", "api/v1/autocomplete/configs", false)

	config.AddCommand(addConfig)
	config.AddCommand(listConfig)
	config.AddCommand(deleteConfig)

	confluence.AddCommand(config)

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your Confluence account")
	confluence.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your Confluence account")
	confluence.AddCommand(disconnect)

	subscribe := model.NewAutocompleteData("subscribe", "", "Subscribe the current channel to notifications from Confluence")
	confluence.AddCommand(subscribe)

	unsubscribe := model.NewAutocompleteData("unsubscribe", "[name]", "Unsubscribe the current channel from notifications associated with the given subscription name")
	unsubscribe.AddDynamicListArgument("name", "api/v1/autocomplete/channel-subscriptions", false)
	confluence.AddCommand(unsubscribe)

	list := model.NewAutocompleteData("list", "", "List all subscriptions for the current channel")
	confluence.AddCommand(list)

	edit := model.NewAutocompleteData("edit", "[name]", "Edit the subscription settings associated with the given subscription name")
	edit.AddDynamicListArgument("name", "api/v1/autocomplete/channel-subscriptions", false)
	confluence.AddCommand(edit)

	if showMigrateCommands {
		migrate := model.NewAutocompleteData("migrate", "", "Migrate your subscriptions to a newer version of confluence plugin")
		migrateItems := []model.AutocompleteListItem{{
			HelpText: "List all the old subscriptions to be migrated",
			Item:     "list",
		}, {
			HelpText: "Start the migration of old subscriptions",
			Item:     "start",
		}, {
			HelpText: "Delete all the old subscriptions",
			Item:     "cleanup",
		}}
		migrate.AddStaticListArgument("", false, migrateItems)
		confluence.AddCommand(migrate)
	}

	help := model.NewAutocompleteData("help", "", "Show confluence slash command help")
	confluence.AddCommand(help)

	return confluence
}

func executeConfluenceDefault(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	out := invalidCommand + "\n\n"
	out += getFullHelpText(p, context, args...)

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         out,
	}
}

func (p *Plugin) postCommandResponse(context *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.conf.botUserID,
		ChannelId: context.ChannelId,
		Message:   text,
	}
	_ = config.Mattermost.SendEphemeralPost(context.UserId, post)
}

func (p *Plugin) responsef(commandArgs *model.CommandArgs, format string, args ...interface{}) *model.CommandResponse {
	p.postCommandResponse(commandArgs, fmt.Sprintf(format, args...))
	return &model.CommandResponse{}
}

func (p *Plugin) respondCommandTemplate(commandArgs *model.CommandArgs, path string, values interface{}) *model.CommandResponse {
	t := p.templates[path]
	if t == nil {
		return p.responsef(commandArgs, "no template found for "+path)
	}
	bb := &bytes.Buffer{}
	err := t.Execute(bb, values)
	if err != nil {
		p.responsef(commandArgs, "failed to format results: %v", err)
	}
	return p.responsef(commandArgs, bb.String())
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

func (p *Plugin) help(args *model.CommandArgs) *model.CommandResponse {
	authorized := utils.IsSystemAdmin(args.UserId)

	helpText := helpTextHeader + commonHelpText
	if authorized {
		helpText += sysAdminHelpText
	}

	oldSubscriptions, getErr := service.GetOldSubscriptions()
	if getErr != nil {
		return p.responsef(args, getErr.Error())
	}

	if len(oldSubscriptions) != 0 {
		helpText += migrationCommandsHelpText
	}

	p.postCommandResponse(args, helpText)
	return &model.CommandResponse{}
}

func showInstallCloudHelp(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}
	if len(args) == 0 {
		response := executeConfluenceDefault(p, context, args...)
		return response
	}

	confluenceURL, instance, err := p.installCloudInstance(args[0])
	if err != nil {
		return p.responsef(context, err.Error())
	}

	return p.respondCommandTemplate(context, "/command/install_cloud.md", map[string]string{
		"ConfluenceURL": confluenceURL,
		"RedirectURL":   instance.GetRedirectURL(),
		"CloudURL":      p.GetPluginURL() + p.GetAtlassianConnectURLPath(),
	})
}

func addConfig(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	defaultServerURL := ""
	if len(args) != 0 {
		defaultServerURL = args[0]
	}

	elements := []model.DialogElement{
		{
			DisplayName: configServerURL,
			Name:        configServerURL,
			Type:        "text",
			Default:     defaultServerURL,
			Placeholder: "https://example.com",
			HelpText:    "Please enter your Confluence server URL",
			Optional:    false,
		},
		{
			DisplayName: configClientID,
			Name:        configClientID,
			Type:        "text",
			Placeholder: configClientID,
			HelpText:    "Please enter your Confluence oAuth client ID",
			Optional:    false,
		},
		{
			DisplayName: configClientSecret,
			Name:        configClientSecret,
			Type:        "text",
			Placeholder: configClientSecret,
			HelpText:    "Please enter your Confluence oAuth client Secret",
			Optional:    false,
		},
	}

	requestBody := model.OpenDialogRequest{
		TriggerId: context.TriggerId,
		URL:       fmt.Sprintf(configDialogueEndpoint, p.GetPluginURL(), context.ChannelId, context.UserId),
		Dialog: model.Dialog{
			Title:       configModalTitle,
			CallbackId:  "callbackID",
			SubmitLabel: "Submit",
			Elements:    elements,
		},
	}

	requestPayload, err := json.Marshal(requestBody)
	if err != nil {
		p.responsef(context, err.Error())
	}

	resp, err := http.Post(fmt.Sprintf(configAPIEndpoint, p.GetSiteURL()), "application/json", bytes.NewBuffer(requestPayload))
	if err != nil {
		p.responsef(context, err.Error())
	}
	resp.Body.Close()

	return &model.CommandResponse{}
}

func (p *Plugin) GetConfigKeyList() ([]string, error) {
	page := 0
	var configKeys []string
	for {
		keyList, err := p.API.KVList(page, configPerPage)
		if err != nil {
			return nil, err
		}

		if len(keyList) == 0 {
			break
		}

		var keys []string
		testconfig := ""
		for _, key := range keyList {
			if strings.Contains(key, prefixConfigKey) {
				keys = append(keys, key)
				testconfig += key
			}
		}

		configKeys = append(configKeys, keys...)
		page++
	}
	return configKeys, nil
}

func listConfig(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	configKeys, err := p.GetConfigKeyList()
	if err != nil {
		return p.responsef(context, err.Error())
	}

	if len(configKeys) == 0 {
		p.postCommandResponse(context, noSavedConfig)
		return &model.CommandResponse{}
	}

	confluenceConfig, err := p.instanceStore.LoadSavedConfigs(configKeys)
	if err != nil {
		return p.responsef(context, err.Error())
	}

	p.postCommandResponse(context, serializer.FormattedConfigList(confluenceConfig))
	return &model.CommandResponse{}
}

func deleteConfig(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	if len(args) == 0 {
		return executeConfluenceDefault(p, context, args...)
	}

	instance := strings.Join(args, " ")

	if err := p.instanceStore.DeleteInstanceConfig(instance); err != nil {
		return p.responsef(context, err.Error())
	}

	p.postCommandResponse(context, fmt.Sprintf("Your config is deleted for confluence instance %s", instance))
	return &model.CommandResponse{}
}

func listOldSubscriptions(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	oldSubscriptions, getErr := service.GetOldSubscriptions()
	if getErr != nil {
		p.postCommandResponse(context, getErr.Error())
		return &model.CommandResponse{}
	}

	if len(oldSubscriptions) == 0 {
		p.postCommandResponse(context, noChannelSubscription)
		return &model.CommandResponse{}
	}

	list := serializer.FormattedOldSubscriptionList(oldSubscriptions)
	p.postCommandResponse(context, list)
	return &model.CommandResponse{}
}

func startSubscriptionMigration(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	oldSubscriptions, getErr := service.GetOldSubscriptions()
	if getErr != nil {
		p.postCommandResponse(context, getErr.Error())
		return &model.CommandResponse{}
	}

	if len(oldSubscriptions) == 0 {
		p.postCommandResponse(context, NoOldSubscriptionsMsg)
		return &model.CommandResponse{}
	}

	go func() {
		subscriptionString := p.migrateSubscriptions(oldSubscriptions, context.UserId)
		p.postCommandResponse(context, fmt.Sprintf("%s%s", MigrationCompletedMsg, subscriptionString))
	}()

	p.postCommandResponse(context, MigrationWaitMsg)
	return &model.CommandResponse{}
}

func deleteOldSubscriptions(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}

	oldSubscriptions, getErr := service.GetOldSubscriptions()
	if getErr != nil {
		p.postCommandResponse(context, getErr.Error())
		return &model.CommandResponse{}
	}

	if len(oldSubscriptions) == 0 {
		p.postCommandResponse(context, NoOldSubscriptionsDeleteMsg)
		return &model.CommandResponse{}
	}

	go func() {
		if err := p.API.KVDelete(storePackage.GetOldSubscriptionKey()); err != nil {
			p.API.LogError("Unable to delete old subscriptions", "Error", err.Error())
		}

		p.postCommandResponse(context, CleanupCompletedMsg)
	}()

	p.postCommandResponse(context, CleanupWaitMsg)
	return &model.CommandResponse{}
}

func showInstallServerHelp(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(context.UserId) {
		p.postCommandResponse(context, installOnlySystemAdmin)
		return &model.CommandResponse{}
	}
	if len(args) == 0 {
		response := executeConfluenceDefault(p, context, args...)
		return response
	}

	confluenceURL, instance, err := p.installServerInstance(args[0])
	if err != nil {
		return p.responsef(context, err.Error())
	}

	return p.respondCommandTemplate(context, "/command/install_server.md", map[string]string{
		"ConfluenceURL": confluenceURL,
		"RedirectURL":   instance.GetRedirectURL(),
	})
}

func deleteSubscription(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	if len(args) == 0 {
		p.postCommandResponse(context, specifyAlias)
		return &model.CommandResponse{}
	}
	alias := strings.Join(args, " ")
	if err := p.DeleteSubscription(context.ChannelId, alias, context.UserId); err != nil {
		p.postCommandResponse(context, err.Error())
		return &model.CommandResponse{}
	}
	p.postCommandResponse(context, fmt.Sprintf(subscriptionDeleteSuccess, alias))
	return &model.CommandResponse{}
}

func listChannelSubscription(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	channelSubscriptions, gErr := service.GetSubscriptionsByChannelID(context.ChannelId)
	if gErr != nil {
		p.postCommandResponse(context, gErr.Error())
		return &model.CommandResponse{}
	}

	if len(channelSubscriptions) == 0 {
		p.postCommandResponse(context, noChannelSubscription)
		return &model.CommandResponse{}
	}
	list := serializer.FormattedSubscriptionList(channelSubscriptions)
	p.postCommandResponse(context, list)
	return &model.CommandResponse{}
}

func confluenceHelpCommand(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	helpText := getFullHelpText(p, context, args...)

	p.postCommandResponse(context, helpText)
	return &model.CommandResponse{}
}

func getFullHelpText(p *Plugin, context *model.CommandArgs, args ...string) string {
	helpText := commonHelpText
	if utils.IsSystemAdmin(context.UserId) {
		helpText += sysAdminHelpText
	}

	oldSubscriptions, _ := service.GetOldSubscriptions()
	if len(oldSubscriptions) != 0 {
		helpText += migrationCommandsHelpText
	}

	return helpText
}

func (p *Plugin) executeConnectInfo(userID, confluenceURL string) (types.ID, string) {
	info, err := p.GetUserInfo(types.ID(userID), nil)
	if err != nil {
		return "", fmt.Sprintf("Failed to connect. Error: %v", err)
	}

	if info.Instances.IsEmpty() {
		return "", "No Confluence instances have been installed. Please contact the system administrator."
	}

	if confluenceURL == "" {
		if info.connectable.Len() == 1 {
			confluenceURL = info.connectable.IDs()[0].String()
		}
	}

	instanceID := types.ID(confluenceURL)
	if info.connectable.IsEmpty() {
		return instanceID, fmt.Sprintf("You have already connected all available Confluence accounts. Please use `/confluence disconnect --instance=%s` to disconnect.", instanceID)
	}

	if !info.connectable.Contains(instanceID) {
		return instanceID, fmt.Sprintf("Confluence instance %s is not installed, please contact the system administrator.", instanceID)
	}

	return instanceID, ""
}

func executeConnect(p *Plugin, context *model.CommandArgs, args ...string) *model.CommandResponse {
	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		return p.responsef(context, "Failed to load instances. Error: %v.", err)
	}
	confluenceURL := ""
	if len(args) > 0 {
		confluenceURL = args[0]
	}

	confluenceURL = strings.TrimSuffix(confluenceURL, "/")
	instance := instances.getByAlias(confluenceURL)
	if instance != nil {
		confluenceURL = instance.InstanceID.String()
	}
	isAdmin := len(args) > 1 && strings.EqualFold(args[1], AdminMattermostUserID)

	instanceID, errString := p.executeConnectInfo(context.UserId, confluenceURL)
	if errString != "" {
		return p.responsef(context, errString)
	}

	conn, err := p.userStore.LoadConnection(instanceID, types.ID(context.UserId))
	if err == nil && len(conn.ConfluenceAccountID()) != 0 {
		return p.responsef(context,
			"You already have a Confluence account linked to your Mattermost account from %s. Please use `/confluence disconnect <instance url>` to disconnect.",
			instanceID)
	}
	if _, err = p.instanceStore.LoadInstanceConfig(instanceID.String()); err != nil {
		return p.responsef(context, configNotFoundError, instanceID, instanceID)
	}

	link := instancePathWithAdmin(routeUserConnect, instanceID, isAdmin)
	return p.responsef(context, "[Click here to link your Confluence account](%s%s)", p.GetPluginURL(), link)
}

func executeDisconnect(p *Plugin, commArgs *model.CommandArgs, args ...string) *model.CommandResponse {
	if len(args) > 1 {
		return p.help(commArgs)
	}
	confluenceURL := ""
	if len(args) > 0 {
		confluenceURL = args[0]
	}
	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		return p.responsef(commArgs, "Failed to load instances. Error: %v.", err)
	}
	instance := instances.getByAlias(confluenceURL)
	if instance != nil {
		confluenceURL = instance.InstanceID.String()
	}
	disconnected, err := p.DisconnectUser(confluenceURL, types.ID(commArgs.UserId))
	if errors.Cause(err) == kvstore.ErrNotFound {
		errorStr := "Your account is not connected to Confluence. Please use `/confluence connect <instance url>` to connect your account."
		if confluenceURL != "" {
			errorStr = fmt.Sprintf("You don't have a Confluence account at %s linked to your Mattermost account currently. Please use `/confluence connect` to connect your account.", confluenceURL)
		}
		return p.responsef(commArgs, errorStr)
	}
	if err != nil {
		return p.responsef(commArgs, "Could not complete the **disconnection** request. Error: %v", err)
	}
	return p.responsef(commArgs, "You have successfully disconnected your Confluence account (**%s**).", disconnected.DisplayName)
}

func executeInstanceUninstall(p *Plugin, commArgs *model.CommandArgs, args ...string) *model.CommandResponse {
	if !utils.IsSystemAdmin(commArgs.UserId) {
		return p.responsef(commArgs, "`/confluence uninstall` can only be run by a System Administrator.")
	}
	if len(args) != 2 {
		return p.help(commArgs)
	}

	instanceType := InstanceType(args[0])
	instanceURL := args[1]

	id, err := service.NormalizeConfluenceURL(instanceURL)
	if err != nil {
		return p.responsef(commArgs, err.Error())
	}

	uninstalled, err := p.UninstallInstance(types.ID(id), instanceType)
	if err != nil {
		return p.responsef(commArgs, err.Error())
	}

	uninstallInstructions := `` +
		`Confluence instance successfully uninstalled. Navigate to [**your app management URL**](%s) in order to remove the application from your Confluence instance.
Don't forget to remove Confluence-side webhook in [Confluence System Settings/Webhooks](%s)'
`
	return p.responsef(commArgs, uninstallInstructions, uninstalled.GetManageAppsURL(), uninstalled.GetManageWebhooksURL())
}
