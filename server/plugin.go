package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

const (
	botUserName    = "confluence"
	botDisplayName = "Confluence"
	botDescription = "Bot for confluence plugin."

	documentationURL = "https://github.com/mattermost-community/mattermost-plugin-confluence#readme"
)

type Plugin struct {
	plugin.MattermostPlugin
	client *pluginapi.Client

	BotUserID string

	Router *mux.Router

	flowManager *FlowManager
	forgePoller *ForgePoller

	// templates are loaded on startup
	templates map[string]*template.Template
}

func (p *Plugin) OnActivate() error {
	config.Mattermost = p.API
	p.client = pluginapi.NewClient(p.API, p.Driver)

	if err := p.setUpBotUser(); err != nil {
		config.Mattermost.LogError("Failed to create a bot user", "Error", err.Error())
		return err
	}

	p.Router = p.InitAPI()

	if err := p.OnConfigurationChange(); err != nil {
		return err
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "couldn't get bundle path")
	}

	templates, err := p.loadTemplates(filepath.Join(bundlePath, "assets", "templates"))
	if err != nil {
		return err
	}
	p.templates = templates

	flowManager, err := p.NewFlowManager()
	if err != nil {
		return errors.Wrap(err, "failed to create flow manager")
	}
	p.flowManager = flowManager

	cmd, err := GetCommand(p.API)
	if err != nil {
		return errors.Wrap(err, "failed to get command")
	}

	if err := p.API.RegisterCommand(cmd); err != nil {
		return err
	}

	p.forgePoller = NewForgePoller(p)
	p.forgePoller.Start()

	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.forgePoller != nil {
		p.forgePoller.Stop()
	}
	return nil
}

func (p *Plugin) OnConfigurationChange() error {
	// If OnActivate has not been run yet.
	if config.Mattermost == nil {
		return nil
	}
	var configuration config.Configuration

	if err := config.Mattermost.LoadPluginConfiguration(&configuration); err != nil {
		config.Mattermost.LogError("Error in LoadPluginConfiguration.", "Error", err.Error())
		return err
	}

	if err := configuration.ProcessConfiguration(); err != nil {
		config.Mattermost.LogError("Error in ProcessConfiguration.", "Error", err.Error())
		return err
	}

	forgeSecretMissingWithBridge := configuration.ForgeSharedSecret == "" && strings.TrimSpace(configuration.ForgeDrainURL) != ""

	regenerated := false
	for _, field := range []struct {
		name string
		ptr  *string
	}{
		{"Webhook Secret", &configuration.Secret},
		{"Encryption Key", &configuration.EncryptionKey},
		{"Forge Bridge Shared Secret", &configuration.ForgeSharedSecret},
	} {
		if len(*field.ptr) == 32 {
			continue
		}
		newKey, err := generateRandomKey(32)
		if err != nil {
			config.Mattermost.LogError("Error generating "+field.name+".", "Error", err.Error())
			return err
		}
		*field.ptr = newKey
		config.Mattermost.LogInfo("Auto-generated missing " + field.name + ".")
		regenerated = true
	}
	if forgeSecretMissingWithBridge {
		config.Mattermost.LogError(
			"Auto-generated a new Forge Bridge Shared Secret while a drain URL was already configured. Forge events will be rejected (HTTP 403) until the bridge is re-registered. Re-run `/confluence install cloud` to push the new secret to the Forge bridge.",
		)
	}
	if regenerated {
		if err := p.savePluginConfig(&configuration); err != nil {
			config.Mattermost.LogError("Error saving auto-generated secrets.", "Error", err.Error())
			return err
		}
	}

	if err := configuration.IsValid(); err != nil {
		config.Mattermost.LogError("Error in Validating Configuration.", "Error", err.Error())
		return err
	}

	config.SetConfig(&configuration)
	return nil
}

func generateRandomKey(length int) (string, error) {
	// We need more bytes because base64 encoding expands the size
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	encoded := base64.URLEncoding.EncodeToString(bytes)
	return encoded[:length], nil
}

func (p *Plugin) savePluginConfig(configuration *config.Configuration) error {
	configMap, err := configuration.ToMap()
	if err != nil {
		return errors.Wrap(err, "failed to convert config to map")
	}

	if appErr := p.API.SavePluginConfig(configMap); appErr != nil {
		return errors.Wrap(appErr, "failed to save plugin config")
	}

	return nil
}

func (p *Plugin) setUpBotUser() error {
	botUserID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    botUserName,
		DisplayName: botDisplayName,
		Description: botDescription,
	})
	if err != nil {
		config.Mattermost.LogError("Error in setting up bot user", "Error", err.Error())
		return err
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return err
	}

	profileImage, err := os.ReadFile(filepath.Join(bundlePath, "assets", "icon.png"))
	if err != nil {
		return err
	}

	if appErr := p.API.SetProfileImage(botUserID, profileImage); appErr != nil {
		return errors.Wrap(appErr, "couldn't set profile image")
	}

	config.BotUserID = botUserID
	p.BotUserID = botUserID
	return nil
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, commandArgs *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	args, argErr := util.SplitArgs(commandArgs.Command)
	if argErr != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         argErr.Error(),
		}, nil
	}
	return ConfluenceCommandHandler.Handle(p, commandArgs, args[1:]...), nil
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.API.LogDebug("New request:", "Host", r.Host, "RequestURI", r.RequestURI, "Method", r.Method)

	conf := config.GetConfig()
	if err := conf.IsValid(); err != nil {
		p.API.LogError("This plugin is not configured.", "Error", err.Error())
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	p.Router.ServeHTTP(w, r)
}

func (p *Plugin) debugf(f string, args ...interface{}) {
	p.API.LogDebug(fmt.Sprintf(f, args...))
}

func (p *Plugin) errorf(f string, args ...interface{}) {
	p.API.LogError(fmt.Sprintf(f, args...))
}

func main() {
	plugin.ClientMain(&Plugin{})
}
