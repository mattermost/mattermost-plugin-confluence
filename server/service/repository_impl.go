package service

import "github.com/mattermost/mattermost-plugin-confluence/server/serializer"

// DefaultSubscriptionRepository is the default implementation of SubscriptionRepository
type DefaultSubscriptionRepository struct{}

// NewDefaultSubscriptionRepository creates a new DefaultSubscriptionRepository
func NewDefaultSubscriptionRepository() *DefaultSubscriptionRepository {
	return &DefaultSubscriptionRepository{}
}

// GetSubscriptions returns all subscriptions
func (r *DefaultSubscriptionRepository) GetSubscriptions() (serializer.Subscriptions, error) {
	return GetSubscriptions()
}

// GetSubscriptionsByURLSpaceKey returns subscriptions by URL and space key
func (r *DefaultSubscriptionRepository) GetSubscriptionsByURLSpaceKey(url, spaceKey string) (serializer.StringArrayMap, error) {
	return GetSubscriptionsByURLSpaceKey(url, spaceKey)
}

// GetSubscriptionsByURLPageID returns subscriptions by URL and page ID
func (r *DefaultSubscriptionRepository) GetSubscriptionsByURLPageID(url, pageID string) (serializer.StringArrayMap, error) {
	return GetSubscriptionsByURLPageID(url, pageID)
}
