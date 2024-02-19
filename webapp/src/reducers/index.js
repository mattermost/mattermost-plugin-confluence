import {combineReducers} from 'redux';

import {subscriptionModal, subscriptionEditModal, installedInstances, userConnected} from './subscription_modal';

export default combineReducers({
    subscriptionModal,
    subscriptionEditModal,
    installedInstances,
    userConnected,
});
