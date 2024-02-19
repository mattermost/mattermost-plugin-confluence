package main

import (
	"net/http"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

type UserInfo struct {
	IsConnected bool       `json:"is_connected"`
	CanConnect  bool       `json:"can_connect"`
	User        *User      `json:"user"`
	Instances   *Instances `json:"instances"`

	connectable *Instances
}

func (p *Plugin) httpGetUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		_, _ = respondErr(w, http.StatusMethodNotAllowed,
			errors.New("method "+r.Method+" is not allowed, must be GET"))
	}

	mattermostUserID := r.Header.Get(config.HeaderMattermostUserID)
	if mattermostUserID == "" {
		_, _ = respondErr(w, http.StatusUnauthorized, errors.New("not authorized"))
	}

	info, err := p.GetUserInfo(types.ID(mattermostUserID), nil)
	if err != nil {
		_, _ = respondErr(w, http.StatusInternalServerError, err)
	}

	_, _ = respondJSON(w, info.AsConfigMap())
}

func (p *Plugin) GetUserInfo(mattermostUserID types.ID, user *User) (*UserInfo, error) {
	var err error

	instances, err := p.instanceStore.LoadInstances()
	if err != nil {
		return nil, err
	}

	if user == nil {
		user, err = p.MigrateV2User(mattermostUserID)
		if err != nil {
			return nil, err
		}
	}

	isConnected := !user.ConnectedInstances.IsEmpty()
	connectable := NewInstances()
	for _, instanceID := range instances.IDs() {
		if !user.ConnectedInstances.Contains(instanceID) {
			connectable.Set(instances.Get(instanceID))
		}
	}

	for _, instanceID := range user.ConnectedInstances.IDs() {
		if !instances.Contains(instanceID) {
			user.ConnectedInstances.Delete(instanceID)
		}
	}
	return &UserInfo{
		CanConnect:  !connectable.IsEmpty(),
		IsConnected: isConnected,
		Instances:   instances,
		User:        user,
		connectable: connectable,
	}, nil
}

func (info UserInfo) AsConfigMap() map[string]interface{} {
	m := map[string]interface{}{
		"can_connect":  info.CanConnect,
		"is_connected": info.IsConnected,
	}
	if !info.Instances.IsEmpty() {
		m["instances"] = info.Instances.AsConfigMap()
	}
	if info.User != nil {
		m["user_info"] = info.User.AsConfigMap()
	}
	return m
}
