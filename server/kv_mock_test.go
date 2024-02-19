package main

import (
	"fmt"

	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type testInstance struct {
	InstanceCommon
}

type testClient struct {
	RESTService
}

var _ Instance = (*testInstance)(nil)

const (
	mockInstance1URL = "confluenceurl1"
	mockInstance2URL = "confluenceurl2"
)

var testInstance1 = &testInstance{
	InstanceCommon: InstanceCommon{
		InstanceID: mockInstance1URL,
		IsV2Legacy: true,
		Type:       ServerInstanceType,
	},
}

var testInstance2 = &testInstance{
	InstanceCommon: InstanceCommon{
		InstanceID: mockInstance2URL,
		Type:       "testInstanceType",
	},
}

func (ti testInstance) GetURL() string {
	return ti.InstanceID.String()
}
func (ti testInstance) GetManageAppsURL() string {
	return fmt.Sprintf("%s/apps/manage", ti.InstanceID)
}
func (ti testInstance) GetManageWebhooksURL() string {
	return fmt.Sprintf("%s/webhooks/manage", ti.InstanceID)
}
func (ti testInstance) GetPlugin() *Plugin {
	return ti.Plugin
}
func (ti testInstance) GetMattermostKey() string {
	return "jiraTestInstanceMattermostKey"
}
func (ti testInstance) GetClient(connection *Connection) (Client, error) {
	return testClient{}, nil
}

func (ti testInstance) GetOAuth2Config(isAdmin bool) (*oauth2.Config, error) {
	return &oauth2.Config{}, nil
}

type mockUserStore struct{}

func (store mockUserStore) StoreUser(*User) error {
	return nil
}
func (store mockUserStore) LoadUser(id types.ID) (*User, error) {
	return NewUser(id), nil
}
func (store mockUserStore) StoreConnection(types.ID, types.ID, *Connection) error {
	return nil
}
func (store mockUserStore) LoadConnection(types.ID, types.ID) (*Connection, error) {
	return &Connection{}, nil
}
func (store mockUserStore) LoadMattermostUserID(instanceID types.ID, confluenceUsername string) (types.ID, error) {
	return "testMattermostUserId012345", nil
}
func (store mockUserStore) DeleteConnection(instanceID, mattermostUserID types.ID) error {
	return nil
}
func (store mockUserStore) CountUsers() (int, error) {
	return 0, nil
}
func (store mockUserStore) MapUsers(func(*User) error) error {
	return nil
}

func (store mockUserStore) StoreWebhookID(instanceID, mattermostUserID types.ID, webhookID string) error {
	return nil
}

func (store mockUserStore) LoadWebhookID(instanceID, mattermostUserID types.ID) (string, error) {
	return "testwebhookID12345", nil
}

func (store mockUserStore) DeleteWebhookID(instanceID, mattermostUserID types.ID) error {
	return nil
}

type mockInstanceStore struct{}

func (store mockInstanceStore) DeleteInstance(types.ID) error {
	return nil
}
func (store mockInstanceStore) LoadInstance(types.ID) (Instance, error) {
	return &testInstance{}, nil
}
func (store mockInstanceStore) LoadInstanceFullKey(string) (Instance, error) {
	return &testInstance{}, nil
}
func (store mockInstanceStore) LoadInstances() (*Instances, error) {
	return NewInstances(), nil
}
func (store mockInstanceStore) StoreInstance(instance Instance) error {
	return nil
}
func (store mockInstanceStore) StoreInstances(*Instances) error {
	return nil
}

func (store mockInstanceStore) StoreInstanceConfig(*serializer.ConfluenceConfig) error {
	return nil
}

func (store mockInstanceStore) LoadInstanceConfig(string) (*serializer.ConfluenceConfig, error) {
	return &serializer.ConfluenceConfig{}, nil
}

func (store mockInstanceStore) LoadSavedConfigs([]string) ([]*serializer.ConfluenceConfig, error) {
	return []*serializer.ConfluenceConfig{}, nil
}

func (store mockInstanceStore) DeleteInstanceConfig(string) error {
	return nil
}

type mockOTSStore struct{}

func (store mockOTSStore) StoreOneTimeSecret(token, secret string) error {
	return nil
}

func (store mockOTSStore) LoadOneTimeSecret(token string) (string, error) {
	return "", nil
}

func (store mockOTSStore) VerifyOAuth2State(state string) error {
	return nil
}

func (store mockOTSStore) StoreOAuth2State(state string) error {
	return nil
}
