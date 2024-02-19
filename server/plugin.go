package main

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/bot/logger"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/flow"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/enterprise"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const (
	botUserName    = "confluence"
	botDisplayName = "Confluence"
	botDescription = "Bot for confluence plugin."
)

var regexNonAlphaNum = regexp.MustCompile("[^a-zA-Z0-9]+")

type externalConfig struct {
	Secret string `json:"Secret"`

	// MM roles that can create subscriptions
	RolesAllowedToEditConfluenceSubscriptions string

	// Comma separated list of confluence groups with permission. Empty is all.
	GroupsAllowedToEditConfluenceSubscriptions string
}

type Config struct {
	// externalConfig caches values from the plugin's settings in the server's config.json
	externalConfig

	// user ID of the bot account
	botUserID string

	mattermostSiteURL string

	rsaKey *rsa.PrivateKey
}

type Plugin struct {
	plugin.MattermostPlugin
	client *pluginapi.Client

	// configuration and a muttex to control concurrent access
	conf     Config
	confLock sync.RWMutex

	instanceStore InstanceStore
	userStore     UserStore
	otsStore      OTSStore
	secretStore   SecretStore

	setupFlow *flow.Flow

	// Most of ServeHTTP does not use an http router, but we need one to support
	// the setup wizard flow. Introducing it here incrementally.
	router *mux.Router

	// templates are loaded on startup
	templates map[string]*template.Template

	// telemetry client
	telemetryClient telemetry.Client

	// telemetry Tracker
	tracker telemetry.Tracker

	// service that determines if this Mattermost instance has access to
	// enterprise features
	enterpriseChecker enterprise.Checker
}

func (p *Plugin) OnActivate() error {
	config.Mattermost = p.API
	p.client = pluginapi.NewClient(p.API, p.Driver)

	store := NewStore(p)
	p.instanceStore = store
	p.userStore = store
	p.otsStore = store
	p.secretStore = store
	p.client = pluginapi.NewClient(p.API, p.Driver)
	p.router = mux.NewRouter()

	botUserID, err := p.setUpBotUser()
	if err != nil {
		p.API.LogError("Failed to create a bot user", "Error", err.Error())
		return err
	}

	config.BotUserID = botUserID

	mattermostSiteURL := ""
	ptr := p.API.GetConfig().ServiceSettings.SiteURL
	if ptr != nil {
		mattermostSiteURL = *ptr
	}

	rsaKey, err := p.secretStore.EnsureRSAKey()
	if err != nil {
		return errors.WithMessage(err, "OnActivate: failed to make RSA public key")
	}

	p.updateConfig(func(conf *Config) {
		conf.botUserID = botUserID
		conf.mattermostSiteURL = mattermostSiteURL
		conf.rsaKey = rsaKey
	})

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "couldn't get bundle path")
	}

	templates, err := p.loadTemplates(filepath.Join(bundlePath, "assets", "templates"))
	if err != nil {
		return err
	}
	p.templates = templates

	p.setupFlow = p.NewSetupFlow()

	p.enterpriseChecker = enterprise.NewEnterpriseChecker(p.API)

	// initialize the rudder client once on activation
	p.telemetryClient, err = telemetry.NewRudderClient()
	if err != nil {
		p.API.LogError("Cannot create telemetry client. err=%w", err)
	}

	if err = p.OnConfigurationChange(); err != nil {
		return err
	}

	err = p.registerConfluenceCommand()
	if err != nil {
		p.API.LogError("Cannot register confluence command. err=%w", err)
		return err
	}

	return nil
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	// Load the public configuration fields from the Mattermost server configuration.
	ec := externalConfig{}
	if err := p.API.LoadPluginConfiguration(&ec); err != nil {
		p.API.LogError("Error in LoadPluginConfiguration.", "Error", err.Error())
		return err
	}

	ec.processConfiguration()

	if err := ec.isValid(); err != nil {
		p.API.LogError("Error in Validating Configuration.", "Error", err.Error())
		return err
	}

	p.updateConfig(func(conf *Config) {
		conf.externalConfig = ec
	})

	// OnConfigurationChange is called right before the plugin is activated,
	// In this case there is no requirement of registering the command, as OnActivate will do this as it already have instanceStore.
	if p.instanceStore != nil {
		if err := p.registerConfluenceCommand(); err != nil {
			return err
		}
	}

	// create new tracker on each configuration change
	p.tracker = telemetry.NewTracker(
		p.telemetryClient,
		p.API.GetDiagnosticId(),
		p.API.GetServerVersion(),
		manifest.Id,
		manifest.Version,
		"confluence",
		telemetry.NewTrackerConfig(p.API.GetConfig()),
		logger.New(p.API),
	)
	return nil
}

func (p *Plugin) OnDeactivate() error {
	// close the tracker on plugin deactivation
	if p.telemetryClient != nil {
		err := p.telemetryClient.Close()
		if err != nil {
			return errors.Wrap(err, "onDeactivate: Failed to close telemetryClient")
		}
	}
	return nil
}

func (p *Plugin) OnInstall(c *plugin.Context, event model.OnInstallEvent) error {
	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		return err
	}

	if instances.Len() == 0 {
		return p.setupFlow.ForUser(event.UserId).Start(nil)
	}

	return nil
}

func (c *externalConfig) processConfiguration() {
	c.Secret = strings.TrimSpace(c.Secret)
}

func (c *externalConfig) isValid() error {
	if c.Secret == "" {
		return errors.New("please provide the Webhook Secret")
	}

	return nil
}

func (p *Plugin) setUpBotUser() (string, error) {
	botUserID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    botUserName,
		DisplayName: botDisplayName,
		Description: botDescription,
	})
	if err != nil {
		p.API.LogError("Error in setting up bot user", "Error", err.Error())
		return "", err
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return "", err
	}

	profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "icon.png"))
	if err != nil {
		return "", err
	}

	if appErr := p.API.SetProfileImage(botUserID, profileImage); appErr != nil {
		return "", errors.Wrap(appErr, "couldn't set profile image")
	}

	return botUserID, nil
}

func (p *Plugin) ExecuteCommand(context *plugin.Context, commandArgs *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	args, argErr := utils.SplitArgs(commandArgs.Command)
	if argErr != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         argErr.Error(),
		}, nil
	}
	return ConfluenceCommandHandler.Handle(p, commandArgs, args[1:]...), nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.API.LogDebug("New request:", "Host", r.Host, "RequestURI", r.RequestURI, "Method", r.Method)

	p.InitAPI().ServeHTTP(w, r)
}

func (p *Plugin) debugf(f string, args ...interface{}) {
	p.API.LogDebug(fmt.Sprintf(f, args...))
}

func (p *Plugin) infof(f string, args ...interface{}) {
	p.API.LogInfo(fmt.Sprintf(f, args...))
}

func (p *Plugin) errorf(f string, args ...interface{}) {
	p.API.LogError(fmt.Sprintf(f, args...))
}

func (p *Plugin) GetSiteURL() string {
	return p.getConfig().mattermostSiteURL
}

func (p *Plugin) getConfig() Config {
	p.confLock.RLock()
	defer p.confLock.RUnlock()
	return p.conf
}

func (p *Plugin) updateConfig(f func(conf *Config)) Config {
	p.confLock.Lock()
	defer p.confLock.Unlock()

	f(&p.conf)
	return p.conf
}

func (p *Plugin) GetPluginURLPath() string {
	return "/plugins/" + manifest.Id + "/api/v1"
}

func (p *Plugin) GetAtlassianConnectURLPath() string {
	return "/atlassian-connect.json?secret=" + url.QueryEscape(p.conf.Secret)
}

func (p *Plugin) GetPluginURL() string {
	return strings.TrimRight(p.GetSiteURL(), "/") + p.GetPluginURLPath()
}

func (p *Plugin) GetPluginKey() string {
	sURL := p.GetSiteURL()
	prefix := "mattermost_"
	escaped := regexNonAlphaNum.ReplaceAllString(sURL, "_")

	start := len(escaped) - int(math.Min(float64(len(escaped)), 32))
	return prefix + escaped[start:]
}

func (p *Plugin) track(name, userID string) {
	p.trackWithArgs(name, userID, nil)
}

func (p *Plugin) trackWithArgs(name, userID string, args map[string]interface{}) {
	if args == nil {
		args = map[string]interface{}{}
	}
	args["time"] = model.GetMillis()
	_ = p.tracker.TrackUserEvent(name, userID, args)
}

func main() {
	plugin.ClientMain(&Plugin{})
}
