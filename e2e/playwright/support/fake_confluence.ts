// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createHmac} from 'node:crypto';

import type {ForgeEvent, JournalEntry} from '../fake-confluence/state';

import {fakeConfluenceControlURL, fakeConfluencePort, forgeSharedSecret} from './constants';

async function control(method: string, path: string, body?: unknown) {
    const res = await fetch(`${fakeConfluenceControlURL}/__control${path}`, {
        method,
        headers: {'Content-Type': 'application/json'},
        body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
        throw new Error(`fake-confluence control ${method} ${path} failed: ${res.status} ${await res.text()}`);
    }
    return res.json();
}

function sign(body: string) {
    return createHmac('sha256', forgeSharedSecret).update(body).digest('hex');
}

export const fakeConfluence = {
    reset: () => control('POST', '/reset'),
    requests: (): Promise<JournalEntry[]> => control('GET', '/requests'),
    contractViolations: (): Promise<string[]> => control('GET', '/contract-violations'),
    enqueueForgeEvent: (event: ForgeEvent): Promise<{key: string}> => control('POST', '/forge-event', event),
    pendingForgeEvents: (): Promise<string[]> => control('GET', '/forge-events'),
    fault: (method: string, path: string, status: number) => control('POST', '/fault', {method, path, status}),

    // Calls the drain the way the plugin does, so a test can prove a bad signature is rejected.
    postDrain: async (body: unknown, signature?: string) => {
        const raw = JSON.stringify(body);
        const res = await fetch(`http://127.0.0.1:${fakeConfluencePort}/forge/drain`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json', 'X-MM-Signature': signature ?? sign(raw)},
            body: raw,
        });
        return {status: res.status, body: await res.text()};
    },
};
