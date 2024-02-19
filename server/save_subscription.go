package main

import (
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const (
	subscriptionSaveSuccess = "Your subscription has been saved."
	ParamChannelID          = "channelID"
	ParamSubscriptionType   = "type"
)

func (p *Plugin) handleSaveSubscription(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params[ParamChannelID]
	subscriptionType := params[ParamSubscriptionType]
	userID := r.Header.Get(config.HeaderMattermostUserID)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.LogAndRespondError(w, http.StatusInternalServerError, "Not able to read the request body. ", err)
		return
	}

	statusCode, message, err := p.CreateSubscription(body, channelID, subscriptionType, userID, r.URL.Path)
	if err != nil {
		p.LogAndRespondError(w, statusCode, message, err)
		return
	}

	post := &model.Post{
		UserId:    p.conf.botUserID,
		ChannelId: channelID,
		Message:   message,
	}

	_ = p.API.SendEphemeralPost(userID, post)

	ReturnStatusOK(w)
}

func (p *Plugin) LogAndRespondError(w http.ResponseWriter, statusCode int, errorLog string, err error) {
	p.API.LogError(errorLog, "Error", err.Error())
	http.Error(w, errors.WithMessage(err, errorLog).Error(), statusCode)
}

func (p *Plugin) CreateSubscription(body []byte, channelID, subscriptionType, userID, path string) (int, string, error) {
	instance, err := p.getInstanceFromURL(path)
	if err != nil {
		return http.StatusInternalServerError, "Not able to get instance from url. ", err
	}

	if err = p.HasPermissionToManageSubscription(instance.GetURL(), userID, channelID); err != nil {
		return http.StatusForbidden, "You don't have the permission to create a subscription. Please contact your administrator. ", err
	}

	conn, err := p.userStore.LoadConnection(types.ID(instance.GetURL()), types.ID(userID))
	if err != nil {
		return http.StatusInternalServerError, "Error in loading connection. ", err
	}

	client, err := instance.GetClient(conn)
	if err != nil {
		return http.StatusInternalServerError, "Not able to get Client. ", err
	}

	var subscription serializer.Subscription
	if subscriptionType == serializer.SubscriptionTypeSpace {
		subscription, err = serializer.SpaceSubscriptionFromJSON(body)
		if err != nil {
			return http.StatusBadRequest, "Error decoding request body for space subscription. ", err
		}

		spaceKey := subscription.(*serializer.SpaceSubscription).GetSubscription().SpaceKey
		resp, gErr := client.GetSpaceData(spaceKey)
		if gErr != nil {
			return http.StatusBadRequest, "Error getting space related data for space subscription. ", gErr
		}

		updatedSubscrption := subscription.(*serializer.SpaceSubscription).GetSubscription().UpdateSpaceIDAndUserID(strconv.FormatInt(resp.ID, 10), userID)
		subscription = updatedSubscrption.GetSubscription()
	} else if subscriptionType == serializer.SubscriptionTypePage {
		subscription, err = serializer.PageSubscriptionFromJSON(body)
		if err != nil {
			return http.StatusBadRequest, "Error decoding request body for page subscription. ", err
		}

		pageID, err := strconv.Atoi(subscription.(*serializer.PageSubscription).GetSubscription().PageID)
		if err != nil {
			return http.StatusInternalServerError, "Error converting pageID to integer. ", err
		}

		_, err = client.GetPageData(pageID)
		if err != nil {
			return http.StatusInternalServerError, "Error getting page related data for page subscription. ", err
		}

		updatedSubscrption := subscription.(*serializer.PageSubscription).UpdateUserID(userID)
		subscription = updatedSubscrption.GetSubscription()
	}

	if instance.Common().Type == ServerInstanceType {
		if err := p.CreateWebhook(instance, subscription, userID); err != nil {
			return http.StatusBadRequest, "Not able to create webhook. ", err
		}
	}

	statusCode, sErr := service.SaveSubscription(subscription)
	if sErr != nil {
		return statusCode, "Not able to save the subscription. ", sErr
	}

	return statusCode, subscriptionSaveSuccess, nil
}
