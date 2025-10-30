package service

import "github.com/mattermost/mattermost-plugin-confluence/server/serializer"

//go:generate mockgen -destination=mocks/mock_subscription_repository.go -package=mocks github.com/mattermost/mattermost-plugin-confluence/server/service SubscriptionRepository

// SubscriptionRepository defines the interface for subscription operations
type SubscriptionRepository interface {
	GetSubscriptions() (serializer.Subscriptions, error)
	GetSubscriptionsByURLSpaceKey(url, spaceKey string) (serializer.StringArrayMap, error)
	GetSubscriptionsByURLPageID(url, pageID string) (serializer.StringArrayMap, error)
}

//go:generate mockgen -destination=mocks/mock_store.go -package=mocks github.com/mattermost/mattermost-plugin-confluence/server/service Store

// Store defines the interface for key-value store operations
type Store interface {
	AtomicModify(key string, modify func(initialValue []byte) ([]byte, error)) error
}
