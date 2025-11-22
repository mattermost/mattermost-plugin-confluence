package service

import "github.com/mattermost/mattermost-plugin-confluence/server/store"

// DefaultStore is the default implementation of Store interface
type DefaultStore struct{}

// NewDefaultStore creates a new DefaultStore
func NewDefaultStore() *DefaultStore {
	return &DefaultStore{}
}

// AtomicModify performs an atomic modification on a key-value pair
func (s *DefaultStore) AtomicModify(key string, modify func(initialValue []byte) ([]byte, error)) error {
	return store.AtomicModify(key, modify)
}
