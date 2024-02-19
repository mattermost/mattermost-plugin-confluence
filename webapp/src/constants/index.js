import {id} from '../manifest';

import {ACTION_TYPES} from './action_types';

const CONFLUENCE_PAGE_EVENTS = [
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

const CONFLUENCE_SPACE_EVENTS = [
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
    {
        value: 'space_created',
        label: 'Space Created',
    },
    {
        value: 'space_removed',
        label: 'Space Removed',
    },
    {
        value: 'space_updated',
        label: 'Space Updated',
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

const MATTERMOST_CSRF_COOKIE = 'MMCSRF';
const OPEN_EDIT_SUBSCRIPTION_MODAL_WEBSOCKET_EVENT = `custom_${id}_open_edit_subscription_modal`;
const SPECIFY_ALIAS = 'Please specify a name for the subscription.';

const COMMAND_ADMIN_ONLY = '`/confluence` commands can only be run by a system administrator.';
const SYSTEM_ADMIN_ROLE = 'system_admin';

export default {
    ACTION_TYPES,
    MATTERMOST_CSRF_COOKIE,
    OPEN_EDIT_SUBSCRIPTION_MODAL_WEBSOCKET_EVENT,
    id,
    SPECIFY_ALIAS,
    COMMAND_ADMIN_ONLY,
    SYSTEM_ADMIN_ROLE,
    SUBSCRIPTION_TYPE,
    CONFLUENCE_PAGE_EVENTS,
    CONFLUENCE_SPACE_EVENTS,
};
