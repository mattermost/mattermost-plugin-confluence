package main

import "github.com/mattermost/mattermost/server/public/pluginapi/experimental/flow"

const (
	stepConnected flow.Name = "connected"
)

func (p *Plugin) NewSetupFlow() *flow.Flow {
	pluginURL := *p.client.Configuration.GetConfig().ServiceSettings.SiteURL + "/" + "plugins" + "/" + manifest.Id
	conf := p.getConfig()
	return flow.NewFlow("setup-wizard", p.client, pluginURL, conf.botUserID).
		WithSteps(
			p.stepConnected(),
		).
		InitHTTP(p.router)
}

func (p *Plugin) stepConnected() flow.Step {
	return flow.NewStep(stepConnected).
		WithText("You've successfully connected your Mattermost user account to Confluence.").
		OnRender(p.trackSetupWizard("setup_wizard_user_connect_complete", nil))
}

func (p *Plugin) trackSetupWizard(key string, args map[string]interface{}) func(f *flow.Flow) {
	return func(f *flow.Flow) {
		p.trackWithArgs(key, f.UserID, args)
	}
}
