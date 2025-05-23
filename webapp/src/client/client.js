import request from 'superagent';
import Cookies from 'js-cookie';

import Constants from '../constants';

import manifest from '../manifest';

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
        headers['X-CSRF-Token'] = Cookies.get(Constants.MATTERMOST_CSRF_COOKIE);

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
        headers['X-CSRF-Token'] = Cookies.get(Constants.MATTERMOST_CSRF_COOKIE);

        const response = await request.
            put(url).
            send(body).
            set(headers).
            type('application/json').
            accept('application/json');

        return response.body;
    }
}
