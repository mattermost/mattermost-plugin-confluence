import Constants from '../constants';

export const subscriptionModal = (state = false, action) => {
    switch (action.type) {
    case Constants.ACTION_TYPES.OPEN_SUBSCRIPTION_MODAL:
        return true;
    case Constants.ACTION_TYPES.CLOSE_SUBSCRIPTION_MODAL:
        return false;
    default:
        return state;
    }
};

export const subscriptionEditModal = (state = {}, action) => {
    switch (action.type) {
    case Constants.ACTION_TYPES.RECEIVED_SUBSCRIPTION:
        return {
            alias: action.data.alias,
            baseURL: action.data.baseURL,
            spaceKey: action.data.spaceKey,
            events: action.data.events,
            pageID: action.data.pageID,
            subscriptionType: action.data.subscriptionType,
        };
    case Constants.ACTION_TYPES.CLOSE_SUBSCRIPTION_MODAL:
        return {};
    default:
        return state;
    }
};

export function installedInstances(state = [], action) {
    // We're notified of the instance status at startup (through getConnected)
    // and when we get a websocket instance_status event
    switch (action.type) {
    case Constants.ACTION_TYPES.RECEIVED_INSTANCE_STATUS:
        return action.data.instances ? action.data.instances : [];
    default:
        return state;
    }
}

export function userConnected(state = false, action) {
    switch (action.type) {
    case Constants.ACTION_TYPES.RECEIVED_CONNECTED:
        return action.data.is_connected;
    default:
        return state;
    }
}
