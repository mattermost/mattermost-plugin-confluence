import {PostTypes} from 'mattermost-redux/action_types';

import Client from '../client';
import Constants from '../constants';

export const saveChannelSubscription = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.saveChannelSubscription(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export const editChannelSubscription = (body) => {
    return async () => {
        let data = null;
        try {
            data = await Client.editChannelSubscription(body);
        } catch (error) {
            return {
                data,
                error,
            };
        }

        return {
            data,
            error: null,
        };
    };
};

export function getSubscriptionAccess() {
    return async () => {
        let data = null;
        let error = null;

        try {
            data = await Client.getSubscriptionAccess();
        } catch (e) {
            error = e;
        }

        return {data, error};
    };
}

export function getPluginConfig() {
    return async () => {
        let data = null;
        let error = null;

        try {
            data = await Client.getPluginConfig();
        } catch (e) {
            error = e;
        }

        return {data, error};
    };
}

export const openSubscriptionModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.OPEN_SUBSCRIPTION_MODAL,
    });
};

export const closeSubscriptionModal = () => (dispatch) => {
    dispatch({
        type: Constants.ACTION_TYPES.CLOSE_SUBSCRIPTION_MODAL,
    });
};

export const getChannelSubscription = (channelID, alias, userID) => async (dispatch) => {
    try {
        const response = await Client.getChannelSubscription(channelID, alias);
        dispatch({
            type: Constants.ACTION_TYPES.RECEIVED_SUBSCRIPTION,
            data: response,
        });
    } catch (e) {
        dispatch(sendEphemeralPost(e.response.text, channelID, userID));
    }
};

export function sendEphemeralPost(message, channelID, userID) {
    const timestamp = Date.now();
    const post = {
        id: 'confluencePlugin' + timestamp,
        user_id: userID,
        channel_id: channelID,
        message,
        type: 'system_ephemeral',
        create_at: timestamp,
        update_at: timestamp,
        root_id: '',
        parent_id: '',
        props: {},
    };

    return {
        type: PostTypes.RECEIVED_NEW_POST,
        data: post,
        channelID,
    };
}
