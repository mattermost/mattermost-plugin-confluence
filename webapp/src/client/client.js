import request from 'superagent';

import Constants from '../constants';

import manifest from '../manifest';

const getCookie = (name) => {
    const prefix = `${name}=`;
    const cookie = document.cookie.split('; ').find((c) => c.startsWith(prefix));
    return cookie && decodeURIComponent(cookie.slice(prefix.length));
};

/**
 *  Add web utilities for interacting with servers here
 */
export default class Client {
    constructor() {
        const url = new URL(window.location.href);
        this.baseUrl = url.protocol + '//' + url.host;
        this.pluginUrl = this.baseUrl + '/plugins/' + manifest.id;
        this.apiUrl = this.baseUrl + '/api/v4';
        this.pluginApiUrl = this.pluginUrl + '/api/v1';
    }

    saveChannelSubscription = (channelSubscription) => {
        const url = `${this.pluginApiUrl}/${channelSubscription.channelID}/subscription/${channelSubscription.subscriptionType}`;
        return this.doPost(url, channelSubscription);
    };

    editChannelSubscription = (channelSubscription) => {
        const url = `${this.pluginApiUrl}/${channelSubscription.channelID}/subscription/${channelSubscription.subscriptionType}`;
        return this.doPut(url, channelSubscription);
    };

    getChannelSubscription = (channelID, alias) => {
        const url = `${this.pluginApiUrl}/${channelID}/subscription?alias=${alias}`;
        return this.doGet(url);
    };

    getSubscriptionAccess = () => {
        const url = `${this.pluginApiUrl}/user-connection-info`;
        return this.doGet(url);
    };

    getPluginConfig = () => {
        const url = `${this.pluginApiUrl}/config`;
        return this.doGet(url);
    };

    doGet = async (url, headers = {}) => {
        headers['X-Requested-With'] = 'XMLHttpRequest';

        const response = await request.
            get(url).
            set(headers).
            type('application/json').
            accept('application/json');

        return response.body;
    };

    doPost = async (url, body, headers = {}) => {
        headers['X-Requested-With'] = 'XMLHttpRequest';
        headers['X-CSRF-Token'] = getCookie(Constants.MATTERMOST_CSRF_COOKIE);

        const response = await request.
            post(url).
            send(body).
            set(headers).
            type('application/json').
            accept('application/json');

        return response.body;
    };

    doPut = async (url, body, headers = {}) => {
        headers['X-Requested-With'] = 'XMLHttpRequest';
        headers['X-CSRF-Token'] = getCookie(Constants.MATTERMOST_CSRF_COOKIE);

        const response = await request.
            put(url).
            send(body).
            set(headers).
            type('application/json').
            accept('application/json');

        return response.body;
    }
}
