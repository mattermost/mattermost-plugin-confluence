// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';

import {pageCreatedEvent} from '../fake-confluence/state';

import {instanceURL, mattermostURL, pageId, pluginId, spaceKey, webhookSecret, confluenceFlavor} from './constants';
import {fakeConfluence} from './fake_confluence';
import {waitForPostText} from './ui';

export async function subscribeToTeamSpace(userClient: Client4, channelId: string, alias = 'team-space') {
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/${channelId}/subscription/space_subscription`, {
        method: 'POST',
        headers: {Authorization: `Bearer ${userClient.getToken()}`, 'Content-Type': 'application/json'},
        body: JSON.stringify({
            alias,
            baseURL: instanceURL,
            spaceKey,
            events: ['page_created', 'page_updated', 'page_trashed', 'page_restored', 'comment_created', 'comment_updated'],
            channelID: channelId,
            subscriptionType: 'space_subscription',
        }),
    });
    if (!res.ok) {
        throw new Error(`subscribe failed: ${res.status} ${await res.text()}`);
    }
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
