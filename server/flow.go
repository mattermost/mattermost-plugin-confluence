package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/flow"
)

type FlowManager struct {
	client              *pluginapi.Client
	plugin              *Plugin
	pluginID            string
	botUserID           string
	router              *mux.Router
	getConfiguration    func() *config.Configuration
	MMSiteURL           string
	GetRedirectURL      func() string
	webhookURL          string
	setupFlow           *flow.Flow
	cloudFlow           *flow.Flow
	completionFlow      *flow.Flow
	cloudCompletionFlow *flow.Flow
	announcementFlow    *flow.Flow
}

func (p *Plugin) NewFlowManager() (*FlowManager, error) {
	webhookURL := util.GetPluginURL() + util.GetConfluenceServerWebhookURLPath()

	fm := &FlowManager{
		client:           p.client,
		plugin:           p,
		pluginID:         manifest.Id,
		botUserID:        p.BotUserID,
		router:           p.Router,
		webhookURL:       webhookURL,
		getConfiguration: config.GetConfig,
		MMSiteURL:        util.GetSiteURL(),
		GetRedirectURL:   p.GetRedirectURL,
	}

	setupFlow, err := fm.newFlow("setup")
	if err != nil {
		p.client.Log.Error("Error creating new flow for setup", "error", err.Error())
		return nil, err
	}
	setupFlow.WithSteps(
		fm.stepWelcome(),
		fm.stepInstanceURL(),
		fm.stepEditionQuestion(),
		fm.stepCloudOAuthConfigure(),
		fm.stepCloudForgeBridge(),
		fm.stepServerVersionQuestion(),
		fm.stepCSversionGreaterthan9(),
		fm.stepCSversionLessthan9(),
		fm.stepOAuthInput(),
		fm.stepOAuthConnect(),
		fm.stepAnnouncementQuestion(),
		fm.stepAnnouncementConfirmation(),
		fm.stepDone(),
		fm.stepCancel("install <instance-type>"),
	)
	fm.setupFlow = setupFlow

	cloudFlow, err := fm.newFlow("cloud-setup")
	if err != nil {
		p.client.Log.Error("Error creating cloud setup flow", "error", err.Error())
		return nil, err
	}
	cloudFlow.WithSteps(
		fm.stepCloudWelcome(),
		fm.stepCloudInstanceURL(),
		fm.stepCloudOAuthConfigure(),
		fm.stepCloudForgeBridge(),
		fm.stepOAuthConnect(),
		fm.stepDone(),
		fm.stepCancel("install cloud"),
	)
	fm.cloudFlow = cloudFlow

	completionFlow, err := fm.newFlow("completion")
	if err != nil {
		p.client.Log.Error("Error creating new flow for completion", "error", err.Error())
		return nil, err
	}
	completionFlow.WithSteps(
		fm.stepWebhookInstructions(),
		fm.stepDone(),
		fm.stepCancel("completion"),
	)
	fm.completionFlow = completionFlow

	cloudCompletionFlow, err := fm.newFlow("cloud-completion")
	if err != nil {
		p.client.Log.Error("Error creating new flow for cloud completion", "error", err.Error())
		return nil, err
	}
	cloudCompletionFlow.WithSteps(
		fm.stepCloudCompletionDone(),
		fm.stepCancel("cloud-completion"),
	)
	fm.cloudCompletionFlow = cloudCompletionFlow

	announcementFlow, err := fm.newFlow("announcement")
	if err != nil {
		p.client.Log.Error("Error creating new flow for announcement", "error", err.Error())
		return nil, err
	}
	announcementFlow.WithSteps(
		fm.stepAnnouncementQuestion(),
		fm.stepAnnouncementConfirmation().Terminal(),

		fm.stepCancel("setup announcement"),
	)
	fm.announcementFlow = announcementFlow

	return fm, nil
}

func (fm *FlowManager) newFlow(name flow.Name) (*flow.Flow, error) {
	flow, err := flow.NewFlow(
		name,
		fm.client,
		fm.pluginID,
		fm.botUserID,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create flow %s", name)
	}

	flow.InitHTTP(fm.router)

	return flow, nil
}

const (
	stepWelcome                  flow.Name = "welcome"
	stepEditionQuestion          flow.Name = "edition-question"
	stepCloudOAuthConfigure      flow.Name = "cloud-oauth-configure"
	stepCloudForgeBridge         flow.Name = "cloud-forge-bridge"
	stepServerVersionQuestion    flow.Name = "server-verstion-question"
	stepConfluenceURL            flow.Name = "confluence-url"
	stepOAuthInput               flow.Name = "oauth-input"
	stepCSversionLessthan9       flow.Name = "server-version-less-than-9"
	stepCSversionGreaterthan9    flow.Name = "server-version-greater-than-9"
	stepWebhookInstructions      flow.Name = "webhook-instruction"
	stepAnnouncementQuestion     flow.Name = "announcement-question"
	stepAnnouncementConfirmation flow.Name = "announcement-confirmation"
	stepDone                     flow.Name = "done"
	stepCancel                   flow.Name = "cancel"
	stepOAuthConnect             flow.Name = "oauth-connect"

	keyConfluenceURL     = "ConfluenceURL"
	keyIsOAuthConfigured = "IsOAuthConfigured"
	keyOAuthCompleteURL  = "OAuthCompleteURL"
	keyForgeInstallURL   = "ForgeInstallURL"
)

const confluenceCloudScopes = "read:confluence-user, read:confluence-content.summary, read:confluence-content.all, read:confluence-space.summary, write:confluence-content"

func cancelButton() flow.Button {
	return flow.Button{
		Name:    "Cancel setup",
		Color:   flow.ColorDanger,
		OnClick: flow.Goto(stepCancel),
	}
}

func (fm *FlowManager) stepCancel(command string) flow.Step {
	return flow.NewStep(stepCancel).
		Terminal().
		WithText(fmt.Sprintf("Confluence integration setup has stopped. Restart setup later by running `/confluence %s`. Learn more about the plugin [here](%s).", command, documentationURL)).
		WithColor(flow.ColorDanger)
}

func continueButtonF(f func(f *flow.Flow) (flow.Name, flow.State, error)) flow.Button {
	return flow.Button{
		Name:    "Continue",
		Color:   flow.ColorPrimary,
		OnClick: f,
	}
}

func continueButton(next flow.Name) flow.Button {
	return continueButtonF(flow.Goto(next))
}

func (fm *FlowManager) getBaseState() flow.State {
	cfg := fm.getConfiguration()
	isOAuthConfigured := cfg.ConfluenceOAuthClientID != "" || cfg.ConfluenceOAuthClientSecret != ""
	return flow.State{
		keyConfluenceURL:     cfg.GetConfluenceBaseURL(),
		keyIsOAuthConfigured: isOAuthConfigured,
		keyOAuthCompleteURL:  util.GetPluginURL() + routeUserComplete,
		keyForgeInstallURL:   cfg.GetForgeInstallURL(),
	}
}

func (fm *FlowManager) StartSetupWizard(userID string, delegatedFrom string) error {
	state := fm.getBaseState()

	err := fm.setupFlow.ForUser(userID).Start(state)
	if err != nil {
		fm.plugin.client.Log.Error("Error creating setup flow for user", "UserID", userID, "error", err.Error())
		return err
	}

	fm.client.Log.Debug("Started setup wizard", "userID", userID, "delegatedFrom", delegatedFrom)

	return nil
}

func (fm *FlowManager) StartCloudSetupWizard(userID string) error {
	state := fm.getBaseState()
	if err := fm.cloudFlow.ForUser(userID).Start(state); err != nil {
		fm.plugin.client.Log.Error("Error starting cloud setup flow", "UserID", userID, "error", err.Error())
		return err
	}
	return nil
}

func (fm *FlowManager) StartCompletionWizard(userID string) error {
	state := fm.getBaseState()

	wizard := fm.completionFlow
	if config.GetConfig().IsCloud {
		wizard = fm.cloudCompletionFlow
	}

	if err := wizard.ForUser(userID).Start(state); err != nil {
		fm.plugin.client.Log.Error("Error creating setup flow for user", "UserID", userID, "error", err.Error())
		return err
	}

	fm.client.Log.Debug("Started setup wizard", "userID", userID)

	return nil
}

func (fm *FlowManager) stepWelcome() flow.Step {
	welcomeText := fmt.Sprintf(":wave: Welcome to your Confluence integration! [Learn more](%s)", documentationURL)
	welcomePretext := "Just a few configuration steps to go!"

	return flow.NewStep(stepWelcome).
		WithText(welcomeText).
		WithPretext(welcomePretext).
		WithButton(continueButton(stepConfluenceURL))
}

func (fm *FlowManager) stepServerVersionQuestion() flow.Step {
	delegateQuestionText := "Are you using Confluence server version greater than or equal to 9?"
	return flow.NewStep(stepServerVersionQuestion).
		WithText(delegateQuestionText).
		WithButton(flow.Button{
			Name:  "Yes",
			Color: flow.ColorPrimary,
			OnClick: func(_ *flow.Flow) (flow.Name, flow.State, error) {
				pluginConfig := config.GetConfig()
				pluginConfig.ServerVersionGreaterthan9 = true
				config.SetConfig(pluginConfig)

				return stepCSversionGreaterthan9, nil, nil
			},
		}).
		WithButton(flow.Button{
			Name:  "No",
			Color: flow.ColorDefault,
			OnClick: func(_ *flow.Flow) (flow.Name, flow.State, error) {
				pluginConfig := config.GetConfig()
				pluginConfig.ServerVersionGreaterthan9 = false
				config.SetConfig(pluginConfig)

				return stepCSversionLessthan9, nil, nil
			},
		})
}

func (fm *FlowManager) stepCSversionGreaterthan9() flow.Step {
	return flow.NewStep(stepCSversionGreaterthan9).
		WithText(
			fmt.Sprintf(
				"%s has been successfully added. To finish the configuration, add an Application Link in your Confluence instance following these steps:\n",
				fm.getConfluenceBaseURL(),
			) +
				"1. Go to [**Settings > Applications > Application Links**]({{ .ConfluenceURL }}/plugins/servlet/applinks/listApplicationLinks)\n" +
				"   ![image](https://user-images.githubusercontent.com/90389917/202149868-a3044351-37bc-43c0-9671-aba169706917.png)\n" +
				"2. Select **Create link**.\n" +
				"3. On the **Create Link** screen, select **External Application** and **Incoming** as `Application type` and `Direction` respectively. Select **Continue**.\n" +
				"4. On the **Link Applications** screen, set the following values:\n" +
				"   - **Name**: `Mattermost`\n" +
				fmt.Sprintf("   - **Redirect URL**: `%s`\n", fm.GetRedirectURL()) +
				"   - **Application Permissions**: `Admin`\n" +
				"   Select **Continue**.\n" +
				"5. Copy the `clientID` and `clientSecret` from **Settings**.",
		).
		WithButton(continueButton(stepOAuthInput))
}

func (fm *FlowManager) stepWebhookInstructions() flow.Step {
	return flow.NewStep(stepWebhookInstructions).
		WithText(
			"You have successfully connected your Mattermost account to Confluence server. To finish the configuration, add a Webhook in your Confluence server following these steps:\n" +
				"1. Go to [**Settings > Plugins > Servlet > Webhooks**]({{ .ConfluenceURL }}/plugins/servlet/webhooks/)\n" +
				"2. Select **Create Webhook**.\n" +
				"4. On the **Create Webhook** screen, set the following values:\n" +
				"   - **Name**: `Mattermost Webhook`\n" +
				fmt.Sprintf("   - **URL**: `%s`\n", fm.webhookURL) +
				"   - Select all the Events in the list\n" +
				"   Select **Save**.\n",
		).
		WithButton(continueButton(stepDone))
}

func (fm *FlowManager) stepCSversionLessthan9() flow.Step {
	return flow.NewStep(stepCSversionLessthan9).
		WithText(fmt.Sprintf(`
To configure the plugin, create a new app in your [Confluence Server](%s) following these steps:
1. Navigate to **Settings > Apps > Manage Apps**. For older versions of Confluence, navigate to **Administration > Applications > Add-ons > Manage add-ons**.
2. Choose **Settings** at the bottom of the page, enable development mode, and apply the change. Development mode allows you to install apps from outside of the Atlassian Marketplace.
3. Press **Upload app**.
4. Choose **From my computer** and upload the Mattermost for Confluence OBR file.
5. Once the app is installed, press **Configure** to open the configuration page.
6. In the **Webhook URL** field, enter: %s
7. Press **Save** to finish the setup.
`, fm.getConfluenceBaseURL(), fm.webhookURL)).
		WithButton(continueButton(stepDone))
}

func (fm *FlowManager) stepInstanceURL() flow.Step {
	enterpriseText := "Click the Continue button below to open a dialog to enter the **Confluence URL**"
	return flow.NewStep(stepConfluenceURL).
		WithText(enterpriseText).
		WithButton(flow.Button{
			Name:  "Continue",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:            "Confluence URL",
				IntroductionText: "Enter the **Confluence URL** of your Confluence instance (Example: https://confluence.example.com).",
				SubmitLabel:      "Save & continue",
				Elements: []model.DialogElement{
					{

						DisplayName: "Confluence URL",
						Name:        "confluence_url",
						Type:        "text",
						SubType:     "url",
						Placeholder: "Enter Confluence URL",
					},
				},
			},
			OnDialogSubmit: fm.submitConfluenceURL,
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) submitConfluenceURL(_ *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	errorList := map[string]string{}

	confluenceURLRaw, ok := submitted["confluence_url"]
	if !ok {
		return "", nil, nil, errors.New("confluence_url missing")
	}
	confluenceURL, ok := confluenceURLRaw.(string)
	if !ok {
		return "", nil, nil, errors.New("confluence_url is not a string")
	}

	if _, err := service.CheckConfluenceURL(fm.MMSiteURL, confluenceURL, false); err != nil {
		errorList["confluence_url"] = err.Error()
	}

	if len(errorList) != 0 {
		return "", nil, errorList, nil
	}

	cfg := fm.getConfiguration()
	cfg.ConfluenceURL = confluenceURL
	cfg.IsCloud = false
	cfg.Sanitize()

	configMap, err := cfg.ToMap()
	if err != nil {
		fm.plugin.client.Log.Error("Error converting config to map", "Flow step", stepConfluenceURL, "error", err.Error())
		return "", nil, nil, err
	}

	if err = fm.client.Configuration.SavePluginConfig(configMap); err != nil {
		fm.plugin.client.Log.Error("Error saving the plugin config", "error", err.Error())
		return "", nil, nil, errors.Wrap(err, "failed to save plugin config")
	}

	return stepEditionQuestion, flow.State{
		keyConfluenceURL: cfg.GetConfluenceBaseURL(),
	}, nil, nil
}

func (fm *FlowManager) stepOAuthInput() flow.Step {
	return flow.NewStep(stepOAuthInput).
		WithText("Click the Continue button below to open a dialog to enter the **Application ID** and **Secret**.").
		WithButton(flow.Button{
			Name:  "Continue",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:            "Confluence OAuth Credentials",
				IntroductionText: "Please enter the **Application ID** and **Secret** you copied in a previous step.{{ if .IsOAuthConfigured }}\n\n**Any existing OAuth configuration will be overwritten.**{{end}}",
				SubmitLabel:      "Save & continue",
				Elements: []model.DialogElement{
					{
						DisplayName: "Confluence OAuth Application ID",
						Name:        "client_id",
						Type:        "text",
						SubType:     "text",
						Placeholder: "Enter Confluence OAuth Application ID",
					},
					{
						DisplayName: "Confluence OAuth Secret",
						Name:        "client_secret",
						Type:        "text",
						SubType:     "text",
						Placeholder: "Enter Confluence OAuth Secret",
					},
				},
			},
			OnDialogSubmit: fm.submitOAuthConfig,
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) submitOAuthConfig(_ *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	errorList := map[string]string{}

	clientIDRaw, ok := submitted["client_id"]
	if !ok {
		return "", nil, nil, errors.New("client_id missing")
	}
	clientID, ok := clientIDRaw.(string)
	if !ok {
		return "", nil, nil, errors.New("client_id is not a string")
	}

	clientID = strings.TrimSpace(clientID)

	if len(clientID) < 32 {
		errorList["client_id"] = "Client ID should be at least 32 characters long"
	}

	clientSecretRaw, ok := submitted["client_secret"]
	if !ok {
		return "", nil, nil, errors.New("client_secret missing")
	}
	clientSecret, ok := clientSecretRaw.(string)
	if !ok {
		return "", nil, nil, errors.New("client_secret is not a string")
	}

	clientSecret = strings.TrimSpace(clientSecret)

	if len(clientSecret) < 64 {
		errorList["client_secret"] = "Client Secret should be at least 64 characters long"
	}

	if len(errorList) != 0 {
		return "", nil, errorList, nil
	}

	config := fm.getConfiguration()
	config.ConfluenceOAuthClientID = clientID
	config.ConfluenceOAuthClientSecret = clientSecret

	configMap, err := config.ToMap()
	if err != nil {
		fm.plugin.client.Log.Error("Error converting config to map", "Flow step", stepOAuthInput, "error", err.Error())
		return "", nil, nil, err
	}

	err = fm.client.Configuration.SavePluginConfig(configMap)
	if err != nil {
		fm.plugin.client.Log.Error("Error saving the plugin config", "error", err.Error())
		return "", nil, nil, errors.Wrap(err, "failed to save plugin config")
	}

	return stepOAuthConnect, nil, nil, nil
}

func (fm *FlowManager) stepOAuthConnect() flow.Step {
	connectPretext := "##### :white_check_mark: Connect your Confluence account"
	connectURL := fmt.Sprintf(oauth2ConnectPath, util.GetPluginURL())
	connectText := fmt.Sprintf("Go [here](%s) to connect your account.", connectURL)
	return flow.NewStep(stepOAuthConnect).
		WithText(connectText).
		WithPretext(connectPretext)
}

func (fm *FlowManager) stepAnnouncementQuestion() flow.Step {
	defaultMessage := fmt.Sprintf("Hi team,\n"+
		"\n"+
		"We've set up the Mattermost Confluence plugin to enable notifications from Confluence in Mattermost. To get started, run the `/confluence connect` slash command from any channel within Mattermost to connect that channel with Confluence. See the [documentation](%s) for details on using the Confluence plugin.", documentationURL)

	return flow.NewStep(stepAnnouncementQuestion).
		WithText("Want to let your team know?").
		WithButton(flow.Button{
			Name:  "Send Message",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:       "Notify your team",
				SubmitLabel: "Send message",
				Elements: []model.DialogElement{
					{
						DisplayName: "To",
						Name:        "channel_id",
						Type:        "select",
						Placeholder: "Select channel",
						DataSource:  "channels",
					},
					{
						DisplayName: "Message",
						Name:        "message",
						Type:        "textarea",
						Default:     defaultMessage,
						HelpText:    "You can edit this message before sending it.",
					},
				},
			},
			OnDialogSubmit: fm.submitChannelAnnouncement,
		}).
		WithButton(flow.Button{
			Name:    "Not now",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(stepWebhookInstructions),
		})
}

func (fm *FlowManager) stepAnnouncementConfirmation() flow.Step {
	return flow.NewStep(stepAnnouncementConfirmation).
		WithText("Message to ~{{ .ChannelName }} was sent.").
		Next(stepDone)
}

func (fm *FlowManager) submitChannelAnnouncement(f *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	channelIDRaw, ok := submitted["channel_id"]
	if !ok {
		return "", nil, nil, errors.New("channel_id missing")
	}
	channelID, ok := channelIDRaw.(string)
	if !ok {
		return "", nil, nil, errors.New("channel_id is not a string")
	}

	channel, err := fm.client.Channel.Get(channelID)
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to get channel")
	}

	messageRaw, ok := submitted["message"]
	if !ok {
		return "", nil, nil, errors.New("message is not a string")
	}
	message, ok := messageRaw.(string)
	if !ok {
		return "", nil, nil, errors.New("message is not a string")
	}

	post := &model.Post{
		UserId:    f.UserID,
		ChannelId: channel.Id,
		Message:   message,
	}
	err = fm.client.Post.CreatePost(post)
	if err != nil {
		fm.plugin.client.Log.Error("Error creating the post for channel announcement", "error", err.Error())
		return "", nil, nil, errors.Wrap(err, "failed to create announcement post")
	}

	return stepAnnouncementConfirmation, flow.State{
		"ChannelName": channel.Name,
	}, nil, nil
}

func (fm *FlowManager) stepDone() flow.Step {
	return flow.NewStep(stepDone).
		Terminal().
		WithText(":tada: You successfully installed Confluence.")
}

func (fm *FlowManager) stepCloudCompletionDone() flow.Step {
	return flow.NewStep(stepDone).
		Terminal().
		WithPretext("##### :white_check_mark: Confluence Cloud account connected").
		WithText(":tada: You're all set. Events from Confluence will arrive through the Forge bridge — no webhook configuration needed on your Atlassian side.\n\n" +
			"Run `/confluence subscribe` from any channel to start receiving notifications.")
}

func (fm *FlowManager) getConfluenceBaseURL() string {
	pluginConfig := config.GetConfig()

	return pluginConfig.ConfluenceURL
}

func (fm *FlowManager) stepCloudWelcome() flow.Step {
	welcomeText := fmt.Sprintf(
		":wave: Welcome — let's connect Mattermost to a Confluence **Cloud** site.\n\n"+
			"This sets up two things:\n"+
			"1. **OAuth 2.0** so Mattermost can act on Confluence as your users.\n"+
			"2. A small **Forge bridge app** that delivers Confluence events into Mattermost.\n\n"+
			"[Learn more](%s)",
		documentationURL,
	)
	return flow.NewStep(stepWelcome).
		WithPretext("##### :white_check_mark: Step 1: Confluence Cloud URL").
		WithText(welcomeText).
		WithButton(continueButton(stepConfluenceURL)).
		WithButton(cancelButton())
}

func (fm *FlowManager) stepCloudInstanceURL() flow.Step {
	return flow.NewStep(stepConfluenceURL).
		WithText("Click **Continue** to enter your Confluence Cloud site URL (e.g. `https://acme.atlassian.net/wiki`).").
		WithButton(flow.Button{
			Name:  "Continue",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:            "Confluence Cloud URL",
				IntroductionText: "Enter your Confluence Cloud site URL.",
				SubmitLabel:      "Save & continue",
				Elements: []model.DialogElement{
					{
						DisplayName: "Confluence Cloud URL",
						Name:        "confluence_url",
						Type:        "text",
						SubType:     "url",
						Placeholder: "https://acme.atlassian.net/wiki",
					},
				},
			},
			OnDialogSubmit: fm.submitCloudConfluenceURL,
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) submitCloudConfluenceURL(_ *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	confluenceURL, ok := submitted["confluence_url"].(string)
	if !ok || strings.TrimSpace(confluenceURL) == "" {
		return "", nil, map[string]string{"confluence_url": "Confluence URL is required"}, nil
	}
	confluenceURL = strings.TrimSpace(confluenceURL)

	normalized, err := service.NormalizeConfluenceURL(confluenceURL)
	if err != nil {
		return "", nil, map[string]string{"confluence_url": err.Error()}, nil
	}
	if normalized == strings.TrimSuffix(fm.MMSiteURL, "/") {
		return "", nil, map[string]string{"confluence_url": "This is the Mattermost site URL. Please use the Confluence Cloud URL."}, nil
	}
	confluenceURL = normalized

	cfg := fm.getConfiguration()
	cfg.ConfluenceURL = confluenceURL
	cfg.IsCloud = true
	cfg.Sanitize()
	configMap, err := cfg.ToMap()
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to convert config to map")
	}
	if err = fm.client.Configuration.SavePluginConfig(configMap); err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to save plugin config")
	}

	return stepCloudOAuthConfigure, flow.State{
		keyConfluenceURL: cfg.GetConfluenceBaseURL(),
	}, nil, nil
}

func (fm *FlowManager) stepEditionQuestion() flow.Step {
	return flow.NewStep(stepEditionQuestion).
		WithText("Is this a Confluence **Cloud** site (`*.atlassian.net`) or a self-hosted **Server / Data Center** instance?").
		WithButton(flow.Button{
			Name:    "Cloud",
			Color:   flow.ColorPrimary,
			OnClick: flow.Goto(stepCloudOAuthConfigure),
		}).
		WithButton(flow.Button{
			Name:    "Server / Data Center",
			Color:   flow.ColorDefault,
			OnClick: flow.Goto(stepServerVersionQuestion),
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) stepCloudOAuthConfigure() flow.Step {
	oauthCompleteURL := util.GetPluginURL() + routeUserComplete
	text := fmt.Sprintf("##### :white_check_mark: Register an OAuth 2.0 app in Atlassian\n\n"+
		"1. Open the [Atlassian Developer Console](https://developer.atlassian.com/console/myapps/create-3lo-app/) and create an **OAuth 2.0 integration**.\n"+
		"2. Name it something like `Mattermost Confluence Plugin — <your company>`. Accept the terms and create.\n"+
		"3. In **Permissions**, add the **Confluence API** and configure these scopes:\n"+
		"   %s\n"+
		"   These may be split between **Classic** and **Granular** scopes in the console. You do **not** need to add `offline_access` here — it is not a console-configurable scope; the plugin requests it automatically at the `/authorize` step so refresh tokens are issued.\n"+
		"4. In **Authorization** → **OAuth 2.0 (3LO)** → **Add**, set the Callback URL to:\n"+
		"   `%s`\n"+
		"5. In **Settings**, copy the **Client ID** and **Secret**.\n"+
		"6. In **Distribution** → **Edit**, set **Distribution status** to **Sharing** and save. Without this, only the Atlassian account that created the app can complete the OAuth consent — every other Mattermost user will see *\"you don't have access to this app\"* when running `/confluence connect`.\n"+
		"7. Click **Configure** below and paste the credentials.",
		confluenceCloudScopes, oauthCompleteURL)

	return flow.NewStep(stepCloudOAuthConfigure).
		WithText(text).
		WithButton(flow.Button{
			Name:  "Configure",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:       "Confluence Cloud OAuth 2.0",
				SubmitLabel: "Continue",
				Elements: []model.DialogElement{
					{
						DisplayName: "Confluence Cloud OAuth Client ID",
						Name:        "client_id",
						Type:        "text",
						SubType:     "text",
						HelpText:    "The Client ID from the Atlassian developer console.",
					},
					{
						DisplayName: "Confluence Cloud OAuth Client Secret",
						Name:        "client_secret",
						Type:        "text",
						SubType:     "text",
						HelpText:    "The Client Secret from the Atlassian developer console.",
					},
				},
			},
			OnDialogSubmit: fm.submitCloudOAuthConfig,
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) submitCloudOAuthConfig(_ *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	errorList := map[string]string{}

	clientID, ok := submitted["client_id"].(string)
	if !ok || strings.TrimSpace(clientID) == "" {
		errorList["client_id"] = "Client ID is required"
	}
	clientSecret, ok := submitted["client_secret"].(string)
	if !ok || strings.TrimSpace(clientSecret) == "" {
		errorList["client_secret"] = "Client Secret is required"
	}
	if len(errorList) != 0 {
		return "", nil, errorList, nil
	}

	cfg := fm.getConfiguration()
	cfg.ConfluenceOAuthClientID = strings.TrimSpace(clientID)
	cfg.ConfluenceOAuthClientSecret = strings.TrimSpace(clientSecret)
	cfg.IsCloud = true
	cfg.Sanitize()

	configMap, err := cfg.ToMap()
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to convert config to map")
	}
	if err = fm.client.Configuration.SavePluginConfig(configMap); err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to save plugin config")
	}

	return stepCloudForgeBridge, flow.State{
		keyForgeInstallURL: fm.getConfiguration().GetForgeInstallURL(),
	}, nil, nil
}

func (fm *FlowManager) stepCloudForgeBridge() flow.Step {
	text := "##### :white_check_mark: Install the Forge bridge for event delivery\n\n" +
		"Event subscriptions on Confluence Cloud run through a small [Forge](https://developer.atlassian.com/platform/forge/) app. " +
		"Mattermost ships the bridge as source under [`forge/` in the plugin repo](https://github.com/mattermost/mattermost-plugin-confluence/tree/master/forge) " +
		"and each customer self-hosts it under their own Atlassian developer account.\n\n" +
		"{{if .ForgeInstallURL}}1. Install the bridge on your site: [{{.ForgeInstallURL}}]({{.ForgeInstallURL}}).\n" +
		"{{else}}1. Follow the [self-host runbook](https://github.com/mattermost/mattermost-plugin-confluence/blob/master/forge/README.md) to deploy and install the bridge on your Confluence Cloud site. " +
		"A System Admin can also pre-populate **Forge Bridge Install URL** in System Console → Plugins → Confluence so future installers get a click-through link here.\n" +
		"{{end}}" +
		"2. After installation, open the install log (or run `forge webtrigger` against your tenant) and copy the **drain URL** and **register URL**.\n" +
		"3. Click **Register bridge** below and paste both URLs. The plugin will register itself with the bridge and start polling for events.\n\n" +
		":lock: The shared secret never leaves the plugin — it is sent directly from the server to your bridge's register endpoint."

	return flow.NewStep(stepCloudForgeBridge).
		WithText(text).
		WithButton(flow.Button{
			Name:  "Register bridge",
			Color: flow.ColorPrimary,
			Dialog: &model.Dialog{
				Title:       "Forge bridge URLs",
				SubmitLabel: "Register",
				Elements: []model.DialogElement{
					{
						DisplayName: "Drain URL",
						Name:        "drain_url",
						Type:        "text",
						SubType:     "url",
						HelpText:    "The webtrigger URL labelled `drain` from the Forge install log.",
					},
					{
						DisplayName: "Register URL",
						Name:        "register_url",
						Type:        "text",
						SubType:     "url",
						HelpText:    "The webtrigger URL labelled `register` from the Forge install log. Used once.",
					},
				},
			},
			OnDialogSubmit: fm.submitForgeBridgeURLs,
		}).
		WithButton(cancelButton())
}

func (fm *FlowManager) submitForgeBridgeURLs(_ *flow.Flow, submitted map[string]interface{}) (flow.Name, flow.State, map[string]string, error) {
	errorList := map[string]string{}

	drainURL, _ := submitted["drain_url"].(string)
	registerURL, _ := submitted["register_url"].(string)
	drainURL = strings.TrimSpace(drainURL)
	registerURL = strings.TrimSpace(registerURL)

	if !isForgeWebtriggerURL(drainURL) {
		errorList["drain_url"] = "Must be a Forge web trigger URL (https://<id>.webtrigger.atlassian.app/public/...)"
	}
	if !isForgeWebtriggerURL(registerURL) {
		errorList["register_url"] = "Must be a Forge web trigger URL (https://<id>.webtrigger.atlassian.app/public/...)"
	}
	if len(errorList) != 0 {
		return "", nil, errorList, nil
	}

	cfg := fm.getConfiguration()
	if strings.TrimSpace(cfg.ForgeSharedSecret) == "" {
		return "", nil, nil, errors.New("Forge Bridge Shared Secret is not set on this plugin; reload the plugin to regenerate it")
	}

	urls, err := postForgeRegister(registerURL, cfg.ForgeSharedSecret)
	if err != nil {
		errorList["register_url"] = err.Error()
		return "", nil, errorList, nil
	}

	cfg.ForgeDrainURL = drainURL
	cfg.ForgeRegisterURL = registerURL
	if urls != nil {
		if isForgeWebtriggerURL(urls.Reset) {
			cfg.ForgeResetURL = urls.Reset
		}
		if isForgeWebtriggerURL(urls.Drain) {
			cfg.ForgeDrainURL = urls.Drain
		}
		if isForgeWebtriggerURL(urls.Register) {
			cfg.ForgeRegisterURL = urls.Register
		}
	}
	cfg.Sanitize()
	configMap, err := cfg.ToMap()
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to convert config to map")
	}
	if err = fm.client.Configuration.SavePluginConfig(configMap); err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to save plugin config")
	}

	return stepOAuthConnect, nil, nil, nil
}

// Pinned to prevent secret leak / SSRF if an admin pastes a non-Forge URL.
const forgeWebtriggerHostSuffix = ".webtrigger.atlassian.app"

func isForgeWebtriggerURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" || u.Host == "" {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(u.Host), forgeWebtriggerHostSuffix) {
		return false
	}
	return strings.HasPrefix(u.Path, "/public/")
}

type ForgeWebtriggerURLs struct {
	Drain    string `json:"drain"`
	Register string `json:"register"`
	Reset    string `json:"reset"`
}

type forgeRegisterResponse struct {
	OK   bool                `json:"ok"`
	URLs ForgeWebtriggerURLs `json:"urls"`
}

func postForgeRegister(registerURL, secret string) (*ForgeWebtriggerURLs, error) {
	body, err := json.Marshal(map[string]string{"secret": secret})
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode register payload")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build register request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Refuse redirects so the secret stays on the validated host.
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reach register URL")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var parsed forgeRegisterResponse
		_ = json.Unmarshal(respBody, &parsed) // tolerate older bridges with no urls field
		return &parsed.URLs, nil
	case http.StatusConflict:
		return nil, errors.New("Forge bridge is already registered with a different shared secret. Run `/confluence forge reset` to rotate, or if that fails ask your Forge admin to run `forge invoke -f wipeRegistrationFn -e <env>`, then re-run this wizard.")
	default:
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		snippet := strings.TrimSpace(string(respBody))
		if snippet == "" {
			snippet = resp.Status
		}
		return nil, errors.Errorf("bridge rejected registration (%d): %s", resp.StatusCode, snippet)
	}
}
