// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import {expect} from '@mattermost/playwright-lib';

import {pageCreatedEvent, type ForgeEvent} from '../fake-confluence/state';

import {instanceURL, mattermostURL, pageId, pluginId, spaceKey, webhookSecret, confluenceFlavor} from './constants';
import {fakeConfluence} from './fake_confluence';
import {postText, waitForPostText} from './ui';

export const allEvents = ['page_created', 'page_updated', 'page_trashed', 'page_restored', 'comment_created', 'comment_updated'];

export type SubscriptionOptions = {alias?: string; events?: string[]; page?: string};

export async function subscribe(userClient: Client4, channelId: string, {alias = 'team-space', events = allEvents, page}: SubscriptionOptions = {}) {
    const subscriptionType = page ? 'page_subscription' : 'space_subscription';
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/${channelId}/subscription/${subscriptionType}`, {
        method: 'POST',
        headers: {Authorization: `Bearer ${userClient.getToken()}`, 'Content-Type': 'application/json'},
        body: JSON.stringify({
            alias,
            baseURL: instanceURL,
            ...(page ? {pageID: page} : {spaceKey}),
            events,
            channelID: channelId,
            subscriptionType,
        }),
    });
    if (!res.ok) {
        throw new Error(`subscribe failed: ${res.status} ${await res.text()}`);
    }
}

export function subscribeToTeamSpace(userClient: Client4, channelId: string, alias = 'team-space') {
    return subscribe(userClient, channelId, {alias});
}

export async function postServerWebhook(event: {event: string; userKey: string; pageId: number}) {
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/server/webhook?secret=${encodeURIComponent(webhookSecret)}`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            timestamp: Date.now(),
            event: event.event,
            userKey: event.userKey,
            page: {id: event.pageId},
        }),
    });
    if (!res.ok) {
        throw new Error(`server webhook failed: ${res.status} ${await res.text()}`);
    }
}

export async function publishPage(userClient: Client4, channelId: string, options: {title?: string; page?: string; userKey: string}) {
    const title = options.title ?? 'Release notes';
    const id = options.page ?? pageId;
    if (confluenceFlavor === 'server') {
        await postServerWebhook({event: 'page_created', userKey: options.userKey, pageId: Number(id)});
    } else {
        await fakeConfluence.enqueueForgeEvent(pageCreatedEvent(title, id));
    }
    return waitForPostText(userClient, channelId, title);
}

// Resolves once the plugin has drained and acked the event; its post is created asynchronously after that.
export async function deliverForgeEvent(event: ForgeEvent) {
    const {key} = await fakeConfluence.enqueueForgeEvent(event);
    await expect.poll(async () => (await fakeConfluence.pendingForgeEvents()).includes(key), {timeout: 20_000}).toBe(false);
}

export async function channelPostTexts(client: Client4, channelId: string) {
    const {order, posts} = await client.getPosts(channelId);
    return order.map((id) => postText(posts[id]));
}
