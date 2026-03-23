import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import {openSubscriptionModal, getChannelSubscription} from '../actions';

import {splitArgs} from '../utils';
import {getSubscriptionAccess, sendEphemeralPost} from '../actions/subscription_modal';
import Constants from '../constants';

export default class Hooks {
    constructor(store) {
        this.store = store;
    }

    slashCommandWillBePostedHook = async (message, contextArgs) => {
        let commandTrimmed;
        if (message) {
            commandTrimmed = message.trim();
        }

        if (!commandTrimmed.startsWith('/confluence')) {
            return Promise.resolve({
                message,
                args: contextArgs,
            });
        }

        const state = this.store.getState();
        const user = getCurrentUser(state);

        if (commandTrimmed && commandTrimmed === '/confluence subscribe') {
            const {data: subscriptionAccessData, error} = await getSubscriptionAccess()(this.store.dispatch);

            if (error) {
                this.store.dispatch(sendEphemeralPost(Constants.ERROR_EXECUTING_COMMAND, contextArgs.channel_id, user.id));
                return Promise.resolve({});
            }

            if (!subscriptionAccessData?.can_run_subscribe_command) {
                const errorMsg = subscriptionAccessData?.server_version_greater_than_9 ? Constants.DISCONNECTED_USER : Constants.COMMAND_ADMIN_ONLY;
                this.store.dispatch(sendEphemeralPost(errorMsg, contextArgs.channel_id, user.id));
                return Promise.resolve({});
            }

            openSubscriptionModal()(this.store.dispatch);
            return Promise.resolve({});
        } else if (commandTrimmed && commandTrimmed.startsWith('/confluence edit')) {
            const {data: subscriptionAccessData, error} = await getSubscriptionAccess()(this.store.dispatch);

            if (error) {
                this.store.dispatch(sendEphemeralPost(Constants.ERROR_EXECUTING_COMMAND, contextArgs.channel_id, user.id));
                return Promise.resolve({});
            }

            if (!subscriptionAccessData?.can_run_subscribe_command) {
                const errorMsg = subscriptionAccessData?.server_version_greater_than_9 ? Constants.DISCONNECTED_USER : Constants.COMMAND_ADMIN_ONLY;
                this.store.dispatch(sendEphemeralPost(errorMsg, contextArgs.channel_id, user.id));
                return Promise.resolve({});
            }

            const args = splitArgs(commandTrimmed);
            if (args.length < 3) { // eslint-disable-line
                this.store.dispatch(sendEphemeralPost(Constants.SPECIFY_ALIAS, contextArgs.channel_id, user.id));
            } else {
                const alias = args.slice(2).join(' ');
                getChannelSubscription(contextArgs.channel_id, alias, user.id)(this.store.dispatch);
            }
            return Promise.resolve({});
        }
        return Promise.resolve({
            message,
            args: contextArgs,
        });
    }
}
