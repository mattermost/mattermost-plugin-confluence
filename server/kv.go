package main

import (
	"crypto/md5" // #nosec G501
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/kvstore"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const (
	keyInstances              = "instances/v3"
	keyRSAKey                 = "rsa_key"
	keyTokenSecret            = "token_secret"
	prefixInstance            = "conf_instance_"
	prefixUser                = "user_"
	prefixOneTimeSecret       = "ots_" // + unique key that will be deleted after the first verification
	prefixWebhookKey          = "webhook"
	prefixConfigKey           = "_config"
	expiryStoreTimeoutSeconds = 15 * 60
)

type Store interface {
	InstanceStore
	UserStore
	OTSStore
	SecretStore
}

type InstanceStore interface {
	DeleteInstance(types.ID) error
	LoadInstance(types.ID) (Instance, error)
	LoadInstanceFullKey(string) (Instance, error)
	LoadInstances() (*Instances, error)
	StoreInstance(Instance) error
	StoreInstances(*Instances) error
	StoreInstanceConfig(*serializer.ConfluenceConfig) error
	LoadInstanceConfig(string) (*serializer.ConfluenceConfig, error)
	DeleteInstanceConfig(string) error
	LoadSavedConfigs([]string) ([]*serializer.ConfluenceConfig, error)
}

type UserStore interface {
	LoadUser(types.ID) (*User, error)
	StoreUser(*User) error
	StoreConnection(instanceID, mattermostUserID types.ID, connection *Connection) error
	LoadConnection(instanceID, mattermostUserID types.ID) (*Connection, error)
	LoadMattermostUserID(instanceID types.ID, confluenceUsername string) (types.ID, error)
	DeleteConnection(instanceID, mattermostUserID types.ID) error
	StoreWebhookID(instanceID, mattermostUserID types.ID, webhookID string) error
	LoadWebhookID(instanceID, mattermostUserID types.ID) (string, error)
	DeleteWebhookID(instanceID, mattermostUserID types.ID) error
	CountUsers() (int, error)
	MapUsers(func(user *User) error) error
}

type OTSStore interface {
	StoreOneTimeSecret(token, secret string) error
	LoadOneTimeSecret(token string) (string, error)
	VerifyOAuth2State(state string) error
	StoreOAuth2State(state string) error
}

type SecretStore interface {
	EnsureAuthTokenEncryptionSecret() ([]byte, error)
	EnsureRSAKey() (rsaKey *rsa.PrivateKey, returnErr error)
}

// Number of items to retrieve in KVList operations, made a variable so
// that tests can manipulate
var listPerPage = 100

type store struct {
	plugin *Plugin
}

func NewStore(p *Plugin) Store {
	return &store{plugin: p}
}

func keyWithInstanceID(instanceID, key types.ID) string {
	h := md5.New() // #nosec G401
	fmt.Fprintf(h, "%s/%s", instanceID, key)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func keyWithInstanceIDForConfig(instanceID string) string {
	return fmt.Sprintf("%s/%s", instanceID, prefixConfigKey)
}

func keyForWebhookID(instanceID, key types.ID) string {
	h := md5.New() // #nosec G401
	fmt.Fprintf(h, "%s/%s/%s", instanceID, key, prefixWebhookKey)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func hashkey(prefix, key string) string {
	h := md5.New() // #nosec G401
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%s%x", prefix, h.Sum(nil))
}

func (store store) get(key string, v interface{}) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr, "failed to get from store")
	}()

	data, appErr := store.plugin.API.KVGet(key)
	if appErr != nil {
		return appErr
	}
	if data == nil {
		return kvstore.ErrNotFound
	}

	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}

	return nil
}

func (store store) set(key string, v interface{}) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr, fmt.Sprintf("failed to set the value of given key: %s", key))
	}()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	appErr := store.plugin.API.KVSet(key, data)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (store *store) LoadInstance(instanceID types.ID) (Instance, error) {
	if instanceID == "" {
		return nil, errors.Wrap(kvstore.ErrNotFound, "no instance specified")
	}
	instance, err := store.LoadInstanceFullKey(hashkey(prefixInstance, instanceID.String()))
	if err != nil {
		return nil, errors.Wrap(err, instanceID.String())
	}

	return instance, nil
}

func (store *store) LoadInstanceFullKey(fullkey string) (Instance, error) {
	data, appErr := store.plugin.API.KVGet(fullkey)
	if appErr != nil {
		return nil, appErr
	}
	if data == nil {
		return nil, errors.Wrap(kvstore.ErrNotFound, fullkey)
	}

	// Unmarshal into any of the types just so that we can get the common data
	si := serverInstance{}
	err := json.Unmarshal(data, &si)
	if err != nil {
		return nil, err
	}

	switch si.Type {
	case CloudInstanceType:
		ci := cloudInstance{}
		err = json.Unmarshal(data, &ci)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to unmarshal stored Instance "+fullkey)
		}
		ci.Plugin = store.plugin
		return &ci, nil

	case ServerInstanceType:
		si.Plugin = store.plugin
		return &si, nil
	}

	return nil, fmt.Errorf("confluence instance %s has unsupported type: %s", fullkey, si.Type)
}

func (store *store) StoreInstance(instance Instance) error {
	kv := kvstore.NewStore(kvstore.NewPluginStore(store.plugin.API))
	instance.Common().PluginVersion = manifest.Version
	return kv.Entity(prefixInstance).Store(instance.GetID(), instance)
}

func (store *store) DeleteInstance(id types.ID) error {
	kv := kvstore.NewStore(kvstore.NewPluginStore(store.plugin.API))
	return kv.Entity(prefixInstance).Delete(id)
}

func (store *store) LoadInstances() (*Instances, error) {
	kv := kvstore.NewStore(kvstore.NewPluginStore(store.plugin.API))
	vs, err := kv.ValueIndex(keyInstances, &instancesArray{}).Load()
	if errors.Cause(err) == kvstore.ErrNotFound {
		return NewInstances(), nil
	}
	if err != nil {
		return nil, err
	}
	return &Instances{
		ValueSet: vs,
	}, nil
}

func (store *store) StoreInstances(instances *Instances) error {
	kv := kvstore.NewStore(kvstore.NewPluginStore(store.plugin.API))
	return kv.ValueIndex(keyInstances, &instancesArray{}).Store(instances.ValueSet)
}

func (store *store) StoreInstanceConfig(config *serializer.ConfluenceConfig) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to store config, Confluence Instance:%s", config.ServerURL))
	}()

	if err := store.set(keyWithInstanceIDForConfig(config.ServerURL), config); err != nil {
		return err
	}

	store.plugin.debugf("Stored: config for instance, keys:\n\t %s:", keyWithInstanceIDForConfig(config.ServerURL))
	return nil
}

func (store *store) LoadInstanceConfig(instanceID string) (*serializer.ConfluenceConfig, error) {
	var config serializer.ConfluenceConfig
	if err := store.get(keyWithInstanceIDForConfig(instanceID), &config); err != nil {
		return nil, errors.Wrapf(err,
			"failed to load config for Confluence instance:%q", instanceID)
	}

	return &config, nil
}

func (store *store) LoadSavedConfigs(configKeys []string) ([]*serializer.ConfluenceConfig, error) {
	var configs []*serializer.ConfluenceConfig

	for _, configKey := range configKeys {
		var config serializer.ConfluenceConfig
		if err := store.get(configKey, &config); err != nil {
			return nil, errors.Wrapf(err, "failed to load config for Confluence instance key:%q", configKey)
		}
		configs = append(configs, &config)
	}
	return configs, nil
}

func (store *store) DeleteInstanceConfig(instanceID string) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to delete config for instance: %s", instanceID))
	}()

	config, err := store.plugin.API.KVGet(keyWithInstanceIDForConfig(instanceID))
	if err != nil {
		return err
	}

	if config == nil {
		return errors.New("config not found for instance")
	}

	if appErr := store.plugin.API.KVDelete(keyWithInstanceIDForConfig(instanceID)); appErr != nil {
		return appErr
	}
	store.plugin.debugf("Deleted: config for instance, keys:\n\t %s:", keyWithInstanceIDForConfig(instanceID))
	return nil
}

func UpdateInstances(store InstanceStore, updatef func(instances *Instances) error) error {
	instances, err := store.LoadInstances()
	if errors.Cause(err) == kvstore.ErrNotFound {
		instances = NewInstances()
	} else if err != nil {
		return err
	}
	err = updatef(instances)
	if err != nil {
		return err
	}
	return store.StoreInstances(instances)
}

func (store store) StoreConnection(instanceID, mattermostUserID types.ID, connection *Connection) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to store connection, mattermostUserID:%s, Confluence user:%s", mattermostUserID, connection.DisplayName))
	}()

	connection.PluginVersion = manifest.Version

	err := store.set(keyWithInstanceID(instanceID, mattermostUserID), connection)
	if err != nil {
		return err
	}

	err = store.set(keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), mattermostUserID)
	if err != nil {
		return err
	}

	// Also store AccountID -> mattermostUserID because Confluence Cloud is deprecating the name field
	// https://developer.atlassian.com/cloud/Confluence/platform/api-changes-for-user-privacy-announcement/
	err = store.set(keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), mattermostUserID)
	if err != nil {
		return err
	}

	store.plugin.debugf("Stored: connection, keys:\n\t%s (%s): %+v\n\t%s (%s): %s",
		keyWithInstanceID(instanceID, mattermostUserID), mattermostUserID, connection,
		keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), connection.ConfluenceAccountID(), mattermostUserID)

	return nil
}

func (store store) StoreWebhookID(instanceID, mattermostUserID types.ID, webhookID string) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to store webhookID, Confluence user:%s", mattermostUserID))
	}()

	err := store.set(keyForWebhookID(instanceID, mattermostUserID), webhookID)
	if err != nil {
		return err
	}
	store.plugin.debugf("Stored: webhookID for user, keys:\n\t %s:", keyForWebhookID(instanceID, mattermostUserID))
	return nil
}

func (store store) LoadConnection(instanceID, mattermostUserID types.ID) (*Connection, error) {
	c := &Connection{}
	err := store.get(keyWithInstanceID(instanceID, mattermostUserID), c)
	if err != nil {
		return nil, errors.Wrapf(err,
			"failed to load connection for Mattermost user ID:%q, Confluence:%q", mattermostUserID, instanceID)
	}
	c.PluginVersion = manifest.Version
	return c, nil
}

func (store store) LoadWebhookID(instanceID, mattermostUserID types.ID) (string, error) {
	var webhookID string
	err := store.get(keyForWebhookID(instanceID, mattermostUserID), &webhookID)
	if err != nil {
		return "", errors.Wrapf(err,
			"failed to load webhookID for Confluence user:%q", mattermostUserID)
	}
	return webhookID, nil
}

func (store store) DeleteWebhookID(instanceID, mattermostUserID types.ID) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to delete webhook for user:%s", mattermostUserID))
	}()
	appErr := store.plugin.API.KVDelete(keyForWebhookID(instanceID, mattermostUserID))
	if appErr != nil {
		return appErr
	}
	store.plugin.debugf("Deleted: webhookID for user, keys:\n\t %s:", keyForWebhookID(instanceID, mattermostUserID))
	return nil
}

func (store store) LoadMattermostUserID(instanceID types.ID, confluenceUserNameOrID string) (types.ID, error) {
	mattermostUserID := types.ID("")
	err := store.get(keyWithInstanceID(instanceID, types.ID(confluenceUserNameOrID)), &mattermostUserID)
	if err != nil {
		return "", errors.Wrapf(err,
			"failed to load Mattermost user ID for Confluence user/ID: "+confluenceUserNameOrID)
	}
	return mattermostUserID, nil
}

func (store store) DeleteConnection(instanceID, mattermostUserID types.ID) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to delete user, mattermostUserId:%s", mattermostUserID))
	}()

	c, err := store.LoadConnection(instanceID, mattermostUserID)
	if err != nil {
		return err
	}

	// Check for whether the admin token stored for each confluenceURL is of the current user or not. If it is then delete that admin connection also
	if c.IsAdmin {
		adminConnection, lErr := store.LoadConnection(instanceID, AdminMattermostUserID)
		if lErr != nil {
			return lErr
		}

		// Check if both the tokens are same or not
		if c.OAuth2Token == adminConnection.OAuth2Token {
			if err = store.DeleteConnectionFromKVStore(instanceID, AdminMattermostUserID, c); err != nil {
				return err
			}
		}
	}

	err = store.DeleteConnectionFromKVStore(instanceID, mattermostUserID, c)
	if err != nil {
		return err
	}

	return nil
}

func (store store) DeleteConnectionFromKVStore(instanceID, mattermostUserID types.ID, c *Connection) error {
	appErr := store.plugin.API.KVDelete(keyWithInstanceID(instanceID, mattermostUserID))
	if appErr != nil {
		return appErr
	}

	appErr = store.plugin.API.KVDelete(keyWithInstanceID(instanceID, c.ConfluenceAccountID()))
	if appErr != nil {
		return appErr
	}

	store.plugin.debugf("Deleted: user, keys: %s(%s), %s(%s)",
		mattermostUserID, keyWithInstanceID(instanceID, mattermostUserID),
		c.ConfluenceAccountID(), keyWithInstanceID(instanceID, c.ConfluenceAccountID()))
	return nil
}

func (store store) StoreUser(user *User) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr,
			fmt.Sprintf("failed to store user, mattermostUserId:%s", user.MattermostUserID))
	}()

	user.PluginVersion = manifest.Version

	key := hashkey(prefixUser, user.MattermostUserID.String())
	err := store.set(key, user)
	if err != nil {
		return err
	}

	store.plugin.debugf("Stored: user %s key:%s: connected to:%q", user.MattermostUserID, key, user.ConnectedInstances.IDs())
	return nil
}

func (store store) LoadUser(mattermostUserID types.ID) (*User, error) {
	user := NewUser(mattermostUserID)
	key := hashkey(prefixUser, mattermostUserID.String())
	err := store.get(key, user)
	if err != nil {
		return nil, errors.WithMessage(err,
			fmt.Sprintf("failed to load confluence user for mattermostUserId:%s", mattermostUserID))
	}
	return user, nil
}

func (store store) CountUsers() (int, error) {
	count := 0
	for i := 0; ; i++ {
		keys, appErr := store.plugin.API.KVList(i, listPerPage)
		if appErr != nil {
			return 0, appErr
		}

		for _, key := range keys {
			if strings.HasPrefix(key, prefixUser) {
				count++
			}
		}

		if len(keys) < listPerPage {
			break
		}
	}
	return count, nil
}

func (store store) MapUsers(f func(user *User) error) error {
	for i := 0; ; i++ {
		keys, appErr := store.plugin.API.KVList(i, listPerPage)
		if appErr != nil {
			return appErr
		}

		for _, key := range keys {
			if !strings.HasPrefix(key, prefixUser) {
				continue
			}

			user := NewUser("")
			err := store.get(key, user)
			if err != nil {
				return errors.WithMessage(err, fmt.Sprintf("failed to load confluence user for key:%s", key))
			}

			err = f(user)
			if err != nil {
				return err
			}
		}

		if len(keys) < listPerPage {
			break
		}
	}
	return nil
}

// MigrateV2User migrates a user record from the V2 data model if needed. It
// returns an up-to-date User object either way.
func (p *Plugin) MigrateV2User(mattermostUserID types.ID) (*User, error) {
	user, err := p.userStore.LoadUser(mattermostUserID)
	if errors.Cause(err) != kvstore.ErrNotFound {
		// return the existing key (or error)
		return user, err
	}

	// V3 "user" key does not. Migrate.
	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		return nil, err
	}

	user = NewUser(mattermostUserID)
	for _, instanceID := range instances.IDs() {
		_, err = p.userStore.LoadConnection(instanceID, mattermostUserID)
		if errors.Cause(err) == kvstore.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		user.ConnectedInstances.Set(instances.Get(instanceID))
	}
	err = p.userStore.StoreUser(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (store store) StoreOAuth2State(state string) error {
	appErr := store.plugin.API.KVSetWithExpiry(
		hashkey(prefixOneTimeSecret, state), []byte(state), expiryStoreTimeoutSeconds)
	if appErr != nil {
		return errors.WithMessage(appErr, "failed to store state "+state)
	}
	return nil
}

func (store store) VerifyOAuth2State(state string) error {
	data, appErr := store.plugin.API.KVGet(hashkey(prefixOneTimeSecret, state))
	if appErr != nil {
		return errors.WithMessage(appErr, "failed to load state "+state)
	}

	if string(data) != state {
		return errors.New("invalid oauth state, please try again")
	}

	return nil
}

func (store store) EnsureAuthTokenEncryptionSecret() (secret []byte, returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr, "failed to ensure auth token secret")
	}()

	// nil, nil == NOT_FOUND, if we don't already have a key, try to generate one.
	secret, appErr := store.plugin.API.KVGet(keyTokenSecret)
	if appErr != nil {
		return nil, appErr
	}

	if len(secret) == 0 {
		newSecret := make([]byte, 32)
		_, err := rand.Reader.Read(newSecret)
		if err != nil {
			return nil, err
		}

		appErr = store.plugin.API.KVSet(keyTokenSecret, newSecret)
		if appErr != nil {
			return nil, appErr
		}
		secret = newSecret
		store.plugin.debugf("Stored: auth token secret")
	}

	// If we weren't able to save a new key above, another server must have beat us to it. Get the
	// key from the database, and if that fails, error out.
	if secret == nil {
		secret, appErr = store.plugin.API.KVGet(keyTokenSecret)
		if appErr != nil {
			return nil, appErr
		}
	}

	return secret, nil
}

func (store store) EnsureRSAKey() (rsaKey *rsa.PrivateKey, returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.WithMessage(returnErr, "failed to ensure RSA key")
	}()

	err := store.get(keyRSAKey, &rsaKey)
	if err != nil && errors.Cause(err) != kvstore.ErrNotFound {
		return nil, err
	}

	if rsaKey == nil {
		var newRSAKey *rsa.PrivateKey
		newRSAKey, err = rsa.GenerateKey(rand.Reader, 1024) // #nosec G403
		if err != nil {
			return nil, err
		}

		err = store.set(keyRSAKey, newRSAKey)
		if err != nil {
			return nil, err
		}
		rsaKey = newRSAKey
		store.plugin.debugf("Stored: RSA key")

		// If we weren't able to save a new key above, another server must have beaten us to it. Get the
		// key from the database, and if that fails, error out.
		if err = store.get(keyRSAKey, &rsaKey); err != nil {
			return nil, err
		}
	}

	return rsaKey, nil
}

func (store store) StoreOneTimeSecret(token, secret string) error {
	appErr := store.plugin.API.KVSetWithExpiry(
		hashkey(prefixOneTimeSecret, token), []byte(secret), expiryStoreTimeoutSeconds)
	if appErr != nil {
		return errors.WithMessage(appErr, "failed to store one-time secret "+token)
	}
	return nil
}

func (store store) LoadOneTimeSecret(key string) (string, error) {
	b, appErr := store.plugin.API.KVGet(hashkey(prefixOneTimeSecret, key))
	if appErr != nil {
		return "", errors.WithMessage(appErr, "failed to load one-time secret "+key)
	}

	appErr = store.plugin.API.KVDelete(hashkey(prefixOneTimeSecret, key))
	if appErr != nil {
		return "", errors.WithMessage(appErr, "failed to delete one-time secret "+key)
	}
	return string(b), nil
}
