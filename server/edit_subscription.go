package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/utils/types"
)

const subscriptionEditSuccess = "Your subscription has been edited successfully."

func (p *Plugin) handleEditChannelSubscription(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params[ParamChannelID]
	subscriptionType := params[ParamSubscriptionType]
	oldSubscriptionType := params["oldSubscriptionType"]
	userID := r.Header.Get(config.HeaderMattermostUserID)

	instance, err := p.getInstanceFromURL(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = p.HasPermissionToManageSubscription(instance.GetURL(), userID, channelID)
	if err != nil {
		p.LogAndRespondError(w, http.StatusForbidden, "Don't have the permission to create subscription. Please contact your administrator", err)
		return
	}

	conn, err := p.userStore.LoadConnection(types.ID(instance.GetURL()), types.ID(userID))
	if err != nil {
		p.LogAndRespondError(w, http.StatusInternalServerError, "Error in loading connection.", err)
		return
	}

	client, err := instance.GetClient(conn)
	if err != nil {
		p.LogAndRespondError(w, http.StatusInternalServerError, "Not able to get Client.", err)
		return
	}

	subscription, oldSubscription, logMessage, statusCode, err := p.SubscriptionsFromJSON(r.Body, client, subscriptionType, oldSubscriptionType, userID)
	if err != nil {
		p.LogAndRespondError(w, statusCode, logMessage, err)
		return
	}

	if err := p.DeleteSubscription(subscription.GetChannelID(), oldSubscription.GetAlias(), userID); err != nil {
		p.LogAndRespondError(w, http.StatusBadRequest, "Not able to delete subscription.", err)
		return
	}

	if instance.Common().Type == ServerInstanceType {
		if err := p.CreateWebhook(instance, subscription, userID); err != nil {
			p.LogAndRespondError(w, http.StatusBadRequest, "Not able to create webhook.", err)
			return
		}
	}

	if err := service.EditSubscription(subscription); err != nil {
		p.LogAndRespondError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	post := &model.Post{
		UserId:    p.conf.botUserID,
		ChannelId: channelID,
		Message:   subscriptionEditSuccess,
	}
	_ = config.Mattermost.SendEphemeralPost(userID, post)
	ReturnStatusOK(w)
}

func (p *Plugin) SubscriptionsFromJSON(requestBody io.ReadCloser, client Client, subscriptionType, oldSubscriptionType, userID string) (serializer.Subscription, serializer.Subscription, string, int, error) {
	body, err := ioutil.ReadAll(requestBody)
	if err != nil {
		return nil, nil, "Error reading request body.", http.StatusInternalServerError, err
	}

	var subscription serializer.Subscription
	if subscriptionType == serializer.SubscriptionTypeSpace {
		subscription, err = serializer.SpaceSubscriptionFromJSON(body)
		if err != nil {
			return nil, nil, "Error decoding request body for space subscription.", http.StatusBadRequest, err
		}

		spaceKey := subscription.(*serializer.SpaceSubscription).GetSubscription().SpaceKey
		resp, gErr := client.GetSpaceData(spaceKey)
		if gErr != nil {
			return nil, nil, "Error getting space related data for space subscription.", http.StatusBadRequest, gErr
		}

		newSubscription := subscription.(*serializer.SpaceSubscription).GetSubscription().UpdateSpaceIDAndUserID(strconv.FormatInt(resp.ID, 10), userID)
		subscription = newSubscription.GetSubscription()
	} else if subscriptionType == serializer.SubscriptionTypePage {
		subscription, err = serializer.PageSubscriptionFromJSON(body)
		if err != nil {
			return nil, nil, "Error decoding request body for page subscription.", http.StatusBadRequest, err
		}

		pageID, sErr := strconv.Atoi(subscription.(*serializer.PageSubscription).GetSubscription().PageID)
		if sErr != nil {
			return nil, nil, "Error converting pageID to integer.", http.StatusInternalServerError, sErr
		}

		_, err = client.GetPageData(pageID)
		if err != nil {
			return nil, nil, "Error getting page related data for page subscription.", http.StatusInternalServerError, err
		}

		newSubscription := subscription.(*serializer.PageSubscription).GetSubscription().UpdateUserID(userID)
		subscription = newSubscription.GetSubscription()
	}

	var oldSubscription serializer.Subscription
	if oldSubscriptionType == serializer.SubscriptionTypeSpace {
		oldSubscription, err = serializer.OldSpaceSubscriptionFromJSON(body)
		if err != nil {
			return nil, nil, "Error decoding request body for old space subscription.", http.StatusBadRequest, err
		}
	} else if subscriptionType == serializer.SubscriptionTypePage {
		oldSubscription, err = serializer.OldPageSubscriptionFromJSON(body)
		if err != nil {
			return nil, nil, "Error decoding request body for old page subscription.", http.StatusBadRequest, err
		}
	}

	return subscription, oldSubscription, "", http.StatusOK, nil
}
