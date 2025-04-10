package store

import (
	"bytes" // #nosec G501
	"encoding/json"
	"fmt"
	url2 "net/url"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	prefixOneTimeSecret             = "ots_" // + unique key that will be deleted after the first verification
	ConfluenceSubscriptionKeyPrefix = "confluence_subs"
	expiryStoreTimeoutSeconds       = 15 * 60
	keyTokenSecret                  = "token_secret"
	keyRSAKey                       = "rsa_key"
	prefixUser                      = "user_"
	AdminMattermostUserID           = "admin"
)

var ErrNotFound = errors.New("not found")

// lint is suggesting to rename the function names from `storeConnection` to `Connection` so that when the function is accessed from any other package
// it looks like `store.Connection, but this reduces the readability within the function`

// revive:disable:exported

func GetURLSpaceKeyCombinationKey(url, spaceKey string) string {
	u, _ := url2.Parse(url)
	return fmt.Sprintf("%s/%s/%s",
		url2.PathEscape(ConfluenceSubscriptionKeyPrefix),
		url2.PathEscape(u.Hostname()),
		url2.PathEscape(spaceKey))
}

func GetURLPageIDCombinationKey(url, pageID string) string {
	u, _ := url2.Parse(url)
	return fmt.Sprintf("%s/%s/%s",
		url2.PathEscape(ConfluenceSubscriptionKeyPrefix),
		url2.PathEscape(u.Hostname()),
		url2.PathEscape(pageID))
}

func GetSubscriptionKey() string {
	return util.GetKeyHash(ConfluenceSubscriptionKeyPrefix)
}

// from https://github.com/mattermost/mattermost-plugin-jira/blob/master/server/subscribe.go#L625
func AtomicModify(key string, modify func(initialValue []byte) ([]byte, error)) error {
	readModify := func() ([]byte, []byte, error) {
		initialBytes, appErr := config.Mattermost.KVGet(key)
		if appErr != nil {
			return nil, nil, errors.Wrap(appErr, "unable to read initial value")
		}

		modifiedBytes, err := modify(initialBytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "modification error")
		}

		return initialBytes, modifiedBytes, nil
	}

	var (
		retryLimit     = 5
		retryWait      = 30 * time.Millisecond
		success        = false
		currentAttempt = 0
	)
	for !success {
		initialBytes, newValue, err := readModify()

		if err != nil {
			return err
		}

		var setError *model.AppError
		success, setError = config.Mattermost.KVCompareAndSet(key, initialBytes, newValue)
		if setError != nil {
			return errors.Wrap(setError, "problem writing value")
		}

		if currentAttempt == 0 && bytes.Equal(initialBytes, newValue) {
			return nil
		}

		currentAttempt++
		if currentAttempt >= retryLimit {
			return errors.New("reached write attempt limit")
		}

		time.Sleep(retryWait)
	}

	return nil
}

func keyWithInstanceID(instanceID, key string) string {
	return fmt.Sprintf("%s_%s", instanceID, key)
}

func hashkey(prefix, key string) string {
	return fmt.Sprintf("%s_%s", prefix, key)
}

func get(key string, v interface{}) (returnErr error) {
	data, appErr := config.Mattermost.KVGet(key)
	if appErr != nil {
		return appErr
	}
	if data == nil {
		return ErrNotFound
	}

	if err := json.Unmarshal(data, v); err != nil {
		return err
	}

	return nil
}

func set(key string, v interface{}) (returnErr error) {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if appErr := config.Mattermost.KVSet(key, data); appErr != nil {
		return appErr
	}
	return nil
}

func Load(key string) ([]byte, error) {
	data, appErr := config.Mattermost.KVGet(key)
	if appErr != nil {
		return nil, errors.WithMessage(appErr, "failed plugin KVGet")
	}
	if data == nil {
		return nil, errors.Wrap(ErrNotFound, key)
	}
	return data, nil
}

func StoreOAuth2State(state string) error {
	if appErr := config.Mattermost.KVSetWithExpiry(hashkey(prefixOneTimeSecret, state), []byte(state), expiryStoreTimeoutSeconds); appErr != nil {
		return errors.WithMessage(appErr, "failed to store state "+state)
	}
	return nil
}

func VerifyOAuth2State(state string) error {
	data, appErr := config.Mattermost.KVGet(hashkey(prefixOneTimeSecret, state))
	if appErr != nil {
		return errors.WithMessage(appErr, "failed to load state "+state)
	}

	if string(data) != state {
		return errors.New("invalid oauth state, please try again")
	}

	return nil
}

func StoreConnection(instanceID, mattermostUserID string, connection *types.Connection) (returnErr error) {
	if err := set(keyWithInstanceID(instanceID, mattermostUserID), connection); err != nil {
		return err
	}

	if err := set(keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), mattermostUserID); err != nil {
		return err
	}

	// Also store AccountID -> mattermostUserID because Confluence Cloud is deprecating the name field
	// https://developer.atlassian.com/cloud/Confluence/platform/api-changes-for-user-privacy-announcement/
	if err := set(keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), mattermostUserID); err != nil {
		return err
	}

	config.Mattermost.LogDebug("Stored: connection, keys:\n\t%s (%s): %+v\n\t%s (%s): %s",
		keyWithInstanceID(instanceID, mattermostUserID), mattermostUserID, connection,
		keyWithInstanceID(instanceID, connection.ConfluenceAccountID()), connection.ConfluenceAccountID(), mattermostUserID)

	return nil
}

func GetMattermostUserIDFromConfluenceID(instanceID, confluenceAccountID string) (*string, error) {
	var mmUserID string

	if err := get(keyWithInstanceID(instanceID, confluenceAccountID), &mmUserID); err != nil {
		return nil, err
	}

	return &mmUserID, nil
}

func LoadConnection(instanceID, mattermostUserID string) (*types.Connection, error) {
	c := &types.Connection{}
	if err := get(keyWithInstanceID(instanceID, mattermostUserID), c); err != nil {
		return nil, errors.Wrapf(err,
			"failed to load connection for Mattermost user ID:%q, Confluence:%q", mattermostUserID, instanceID)
	}
	return c, nil
}

func DeleteConnection(instanceID, mattermostUserID string) (returnErr error) {
	c, err := LoadConnection(instanceID, mattermostUserID)
	if err != nil {
		return err
	}

	if err = DeleteConnectionFromKVStore(instanceID, mattermostUserID, c); err != nil {
		return err
	}

	return nil
}

func DeleteConnectionFromKVStore(instanceID, mattermostUserID string, c *types.Connection) error {
	if appErr := config.Mattermost.KVDelete(keyWithInstanceID(instanceID, mattermostUserID)); appErr != nil {
		return appErr
	}

	if appErr := config.Mattermost.KVDelete(keyWithInstanceID(instanceID, c.ConfluenceAccountID())); appErr != nil {
		return appErr
	}

	config.Mattermost.LogDebug("Deleted: user, keys: %s(%s), %s(%s)",
		mattermostUserID, keyWithInstanceID(instanceID, mattermostUserID),
		c.ConfluenceAccountID(), keyWithInstanceID(instanceID, c.ConfluenceAccountID()))
	return nil
}

func LoadUser(mattermostUserID string) (*types.User, error) {
	user := types.NewUser(mattermostUserID)
	key := hashkey(prefixUser, mattermostUserID)
	if err := get(key, user); err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("failed to load confluence user for mattermostUserId:%s", mattermostUserID))
	}
	return user, nil
}

func StoreUser(user *types.User) (returnErr error) {
	key := hashkey(prefixUser, user.MattermostUserID)
	if err := set(key, user); err != nil {
		return err
	}

	config.Mattermost.LogDebug("Stored: user %s key:%s: connected to:%q", user.MattermostUserID, key, user.InstanceURL)
	return nil
}
