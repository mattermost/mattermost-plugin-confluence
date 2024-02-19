import {id} from '../manifest';

export const ACTION_TYPES = {
    OPEN_SUBSCRIPTION_MODAL: id + '_open_subscription_modal',
    CLOSE_SUBSCRIPTION_MODAL: id + '_close_subscription_modal',
    RECEIVED_SUBSCRIPTION: id + '_received_subscription',
    RECEIVED_CONNECTED: id + '_connected',
    RECEIVED_INSTANCE_STATUS: id + '_instance_status',
};
