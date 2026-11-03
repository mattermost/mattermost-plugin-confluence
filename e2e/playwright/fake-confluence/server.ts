// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-console */

import {createHmac, randomBytes, timingSafeEqual} from 'node:crypto';
import {createServer, IncomingMessage, Server, ServerResponse} from 'node:http';

import {ContractValidator} from './contract/validator';
import {cloudId, defaultState, FakeConfluenceState, ForgeEvent, JournalEntry, teamSpace} from './state';

type Ctx = {
    req: IncomingMessage;
    res: ServerResponse;
    url: URL;
    body: string;
    state: FakeConfluenceState;
};

export type FakeConfluence = {
    port: number;
    close: () => Promise<void>;
};

const gateway = /^\/ex\/confluence\/[^/]+(\/wiki\/rest\/api\/.*)$/;

// Confluence REST paths, as they appear in the Cloud spec (`/wiki/rest/api/...`).
// Server calls omit `/wiki`; the Cloud gateway prefixes `/ex/confluence/{cloudId}`.
export function specPath(pathname: string): string | undefined {
    let raw: string | undefined;
    const cloud = pathname.match(gateway);
    if (cloud) {
        raw = cloud[1];
    } else if (pathname.startsWith('/wiki/rest/api/')) {
        raw = pathname;
    } else if (pathname.startsWith('/rest/api/')) {
        raw = `/wiki${pathname}`;
    }
    // The server client calls `/rest/api/user/?key=`, which the Cloud spec lists without the slash.
    return raw?.replace(/\/+$/, '') || undefined;
}

export async function startFakeConfluence({port, siteURL, forgeSecret}: {port: number; siteURL: string; forgeSecret: string}): Promise<FakeConfluence> {
    const state = defaultState();
    const contract = new ContractValidator();

    const handleRequest = async (req: IncomingMessage, res: ServerResponse) => {
        const url = new URL(req.url ?? '/', 'http://fake-confluence');
        const body = await readBody(req);
        const ctx: Ctx = {req, res, url, body, state};

        if (url.pathname.startsWith('/__control/')) {
            handleControl(ctx);
            return;
        }

        if (url.pathname === '/forge/drain') {
            handleDrain(ctx, forgeSecret);
            return;
        }

        const entry = toJournalEntry(req, url, body);
        state.journal.push(entry);

        const normalized = specPath(url.pathname);
        if (normalized && req.method) {
            state.contractViolations.push(...contract.validateRequest(req.method, normalized, entry.body));
            captureResponse(res, (status, payload) => {
                state.contractViolations.push(...contract.validateResponse(req.method ?? '', normalized, status, payload));
            });
        }

        const faultKey = `${req.method} ${url.pathname}`;
        const fault = state.faults.get(faultKey);
        if (fault) {
            sendJSON(res, fault, {message: `fault injected for ${faultKey}`});
            return;
        }

        if (handleOAuth(ctx, siteURL) || handleREST(ctx)) {
            return;
        }

        console.warn(`[fake-confluence] unhandled ${req.method} ${url.pathname}`);
        sendJSON(res, 404, {message: `fake-confluence has no route for ${req.method} ${url.pathname}`});
    };

    const server = createServer((req, res) => {
        handleRequest(req, res).catch((err) => {
            console.error('[fake-confluence] handler failed', err);
            if (!res.headersSent) {
                sendJSON(res, 500, {message: 'fake-confluence handler failed'});
            }
        });
    });

    await new Promise<void>((resolve) => server.listen(port, '0.0.0.0', resolve));
    console.log(`[fake-confluence] listening on :${port}`);
    return {port, close: () => closeServer(server)};
}

function handleControl({req, res, url, body, state}: Ctx) {
    const route = `${req.method} ${url.pathname}`;
    if (route === 'POST /__control/reset') {
        const fresh = defaultState();
        Object.assign(state, fresh);
        state.codes = fresh.codes;
        state.faults = fresh.faults;
        state.forgeEvents = fresh.forgeEvents;
        sendJSON(res, 200, {ok: true});
        return;
    }
    if (route === 'GET /__control/requests') {
        sendJSON(res, 200, state.journal);
        return;
    }
    if (route === 'GET /__control/contract-violations') {
        sendJSON(res, 200, state.contractViolations);
        return;
    }
    if (route === 'POST /__control/fault') {
        const fault = JSON.parse(body) as {method: string; path: string; status: number};
        state.faults.set(`${fault.method} ${fault.path}`, fault.status);
        sendJSON(res, 200, {ok: true});
        return;
    }
    if (route === 'POST /__control/forge-event') {
        const event = JSON.parse(body) as ForgeEvent;
        const key = `evt-${state.nextForgeKey++}`;
        state.forgeEvents.push({key, event});
        sendJSON(res, 200, {key});
        return;
    }
    sendJSON(res, 404, {message: `no control route ${route}`});
}

function handleDrain(ctx: Ctx, secret: string) {
    const {req, res, body, state} = ctx;
    const provided = req.headers['x-mm-signature'];
    const expected = createHmac('sha256', secret).update(body).digest('hex');
    const providedBuf = Buffer.from(typeof provided === 'string' ? provided : '');
    const expectedBuf = Buffer.from(expected);
    if (providedBuf.length !== expectedBuf.length || !timingSafeEqual(providedBuf, expectedBuf)) {
        sendJSON(res, 401, {message: 'invalid signature'});
        return;
    }

    const payload = JSON.parse(body) as {limit?: number; ack?: string[]};
    if (payload.ack?.length) {
        const ack = new Set(payload.ack);
        state.forgeEvents = state.forgeEvents.filter((event) => !ack.has(event.key));
    }
    const events = (payload.limit ?? 0) > 0 ? state.forgeEvents.slice(0, payload.limit).map((event) => ({
        key: event.key,
        value: {event: event.event, context: {}, enqueuedAt: Date.now()},
    })) : [];
    sendJSON(res, 200, {events, nextCursor: null});
}

function handleOAuth(ctx: Ctx, siteURL: string): boolean {
    const {req, res, url, body, state} = ctx;
    if (req.method === 'GET' && (url.pathname === '/authorize' || url.pathname === '/rest/oauth2/latest/authorize')) {
        const code = `code-${randomBytes(8).toString('hex')}`;
        state.codes.set(code, state.user.accountId);
        const redirect = new URL(url.searchParams.get('redirect_uri') ?? '');
        redirect.searchParams.set('code', code);
        redirect.searchParams.set('state', url.searchParams.get('state') ?? '');
        res.writeHead(302, {Location: redirect.toString()}).end();
        return true;
    }

    if (req.method === 'POST' && (url.pathname === '/oauth/token' || url.pathname === '/rest/oauth2/latest/token')) {
        const form = new URLSearchParams(body);
        if (form.get('grant_type') === 'authorization_code' && !state.codes.has(form.get('code') ?? '')) {
            sendJSON(res, 400, {error: 'invalid_grant'});
            return true;
        }
        sendJSON(res, 200, {
            access_token: `acc-${randomBytes(8).toString('hex')}`,
            token_type: 'Bearer',
            refresh_token: `ref-${randomBytes(8).toString('hex')}`,
            expires_in: 3600,
        });
        return true;
    }

    if (req.method === 'GET' && url.pathname === '/oauth/token/accessible-resources') {
        sendJSON(res, 200, [{id: cloudId, url: siteURL, name: 'E2E', scopes: ['read:confluence-user']}]);
        return true;
    }
    return false;
}

function handleREST(ctx: Ctx): boolean {
    const {req, res, url, state} = ctx;
    const path = specPath(url.pathname);
    if (!path || req.method !== 'GET') {
        return false;
    }

    if (path === '/wiki/rest/api/user/current' || path === '/wiki/rest/api/user') {
        sendJSON(res, 200, state.user);
        return true;
    }

    const spaceKey = path.match(/^\/wiki\/rest\/api\/space\/([^/]+)$/)?.[1];
    if (spaceKey) {
        const space = state.spaces.find((item) => item.key === decodeURIComponent(spaceKey));
        if (!space) {
            sendJSON(res, 404, {message: 'space not found'});
            return true;
        }
        sendJSON(res, 200, spaceBody(space));
        return true;
    }

    if (path === '/wiki/rest/api/space' || path === '/wiki/rest/api/space/') {
        sendJSON(res, 200, {
            results: state.spaces.map((space) => ({id: space.id, key: space.key, name: space.name})),
            size: state.spaces.length,
        });
        return true;
    }

    const contentID = path.match(/^\/wiki\/rest\/api\/content\/([^/]+)$/)?.[1];
    if (contentID) {
        const page = state.pages[contentID];
        if (!page) {
            sendJSON(res, 404, {message: 'content not found'});
            return true;
        }
        const space = state.spaces.find((item) => item.key === page.spaceKey) ?? teamSpace;
        sendJSON(res, 200, {
            id: page.id,
            title: page.title,
            type: 'page',
            space: spaceBody(space),
            body: {view: {value: page.body, representation: 'storage'}},
            _links: {webui: `/spaces/${space.key}/pages/${page.id}`},
            history: {createdBy: {username: state.user.username}},
        });
        return true;
    }

    return false;
}

function spaceBody(space: {id: number; key: string; name: string}) {
    return {id: space.id, key: space.key, name: space.name, _links: {webui: `/spaces/${space.key}`}};
}

function captureResponse(res: ServerResponse, check: (status: number, payload: unknown) => void) {
    const end = res.end.bind(res);
    res.end = ((chunk?: unknown, ...rest: unknown[]) => {
        if (typeof chunk === 'string') {
            try {
                check(res.statusCode, JSON.parse(chunk));
            } catch {
                // non-JSON responses have no schema to check
            }
        }
        return (end as (...args: unknown[]) => ServerResponse)(chunk, ...rest);
    }) as typeof res.end;
}

function toJournalEntry(req: IncomingMessage, url: URL, body: string): JournalEntry {
    let parsed: unknown = body;
    try {
        parsed = body ? JSON.parse(body) : undefined;
    } catch {
        // form-encoded OAuth bodies stay as text
    }
    return {method: req.method ?? '', path: url.pathname, query: Object.fromEntries(url.searchParams), body: parsed};
}

function readBody(req: IncomingMessage): Promise<string> {
    return new Promise((resolve, reject) => {
        let data = '';
        req.on('data', (chunk) => {
            data += chunk;
        });
        req.on('end', () => resolve(data));
        req.on('error', reject);
    });
}

function sendJSON(res: ServerResponse, status: number, payload: unknown) {
    res.writeHead(status, {'Content-Type': 'application/json'}).end(JSON.stringify(payload));
}

function closeServer(server: Server) {
    return new Promise<void>((resolve, reject) => server.close((err) => (err ? reject(err) : resolve())));
}
