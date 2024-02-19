package store

import (
	"bytes"
	"fmt"
	url2 "net/url"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils"
)

const ConfluenceSubscriptionKeyPrefix = "confluence_subscription"
const OldConfluenceSubscriptionKeyPrefix = "confluence_subs"

func GetURLSpaceKeyCombinationKey(url, spaceKey string) string {
	u, _ := url2.Parse(url)
	return fmt.Sprintf("%s/%s/%s", ConfluenceSubscriptionKeyPrefix, u.Hostname(), spaceKey)
}

func GetURLPageIDCombinationKey(url, pageID string) string {
	u, _ := url2.Parse(url)
	return fmt.Sprintf("%s/%s/%s", ConfluenceSubscriptionKeyPrefix, u.Hostname(), pageID)
}

func GetSubscriptionKey() string {
	return utils.GetKeyHash(ConfluenceSubscriptionKeyPrefix)
}

func GetOldSubscriptionKey() string {
	return utils.GetKeyHash(OldConfluenceSubscriptionKeyPrefix)
}

// from https://github.com/mattermost/mattermost-plugin-confluence/blob/master/server/subscribe.go#L625
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
