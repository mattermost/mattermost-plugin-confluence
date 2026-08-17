import manifest from '../manifest';

import {ACTION_TYPES} from './action_types';

const CONFLUENCE_EVENTS = [
    {
        value: 'comment_created',
        label: 'Comment Create',
    },
    {
        value: 'comment_updated',
        label: 'Comment Update',
    },
    {
        value: 'comment_removed',
        label: 'Comment Remove',
    },
    {
        value: 'page_created',
        label: 'Page Create',
    },
    {
        value: 'page_updated',
        label: 'Page Update',
    },
    {
        value: 'page_trashed',
        label: 'Page Trash',
    },
    {
        value: 'page_restored',
        label: 'Page Restore',
    },
    {
        value: 'page_removed',
        label: 'Page Remove',
    },
];

const SUBSCRIPTION_TYPE = [
    {
        value: 'space_subscription',
        label: 'Space',
    },
    {
        value: 'page_subscription',
        label: 'Page',
    },
];

const {id} = manifest;
const MATTERMOST_CSRF_COOKIE = 'MMCSRF';
const OPEN_EDIT_SUBSCRIPTION_MODAL_WEBSOCKET_EVENT = `custom_${id}_open_edit_subscription_modal`;
const SPECIFY_ALIAS = 'Please specify a name for the subscription.';

const COMMAND_ADMIN_ONLY = '`/confluence` commands can only be run by a system administrator.';
const SYSTEM_ADMIN_ROLE = 'system_admin';
const DISCONNECTED_USER = 'User not connected. Please use `/confluence connect`.';
const ERROR_EXECUTING_COMMAND = 'An error occurred while executing the command. Please try again later.';

const SUBSCRIBE_DENIED_REASON = {
    ADMIN_ONLY: 'admin_only',
    NOT_CONNECTED: 'not_connected',
};

export default {
    ACTION_TYPES,
    CONFLUENCE_EVENTS,
    MATTERMOST_CSRF_COOKIE,
    OPEN_EDIT_SUBSCRIPTION_MODAL_WEBSOCKET_EVENT,
    id,
    SPECIFY_ALIAS,
    COMMAND_ADMIN_ONLY,
    SYSTEM_ADMIN_ROLE,
    SUBSCRIPTION_TYPE,
    DISCONNECTED_USER,
    ERROR_EXECUTING_COMMAND,
    SUBSCRIBE_DENIED_REASON,
};
