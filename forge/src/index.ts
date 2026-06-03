import api, { route, storage, webTrigger } from '@forge/api';
import { createHmac, timingSafeEqual } from 'crypto';

const QUEUE_PREFIX = 'evt:';
const SECRET_KEY = 'mm.drainSecret';
const REGISTERED_KEY = 'mm.registered';
const MAX_DRAIN_BATCH = 100;

const FORGE_STORAGE_MAX_BYTES = 240 * 1024;
const FORGE_STORAGE_ENVELOPE_HEADROOM = 16 * 1024;

type ForgeEvent = {
    eventType?: string;
    content?: {
        id?: string | number;
        type?: string;
        body?: unknown;
        extensions?: { location?: string };
        space?: { key?: string; name?: string };
    };
};

export const enqueue = async (event: unknown, context: unknown): Promise<void> => {
    const ctx = (context ?? {}) as { cloudId?: string };
    const evt = (event ?? {}) as ForgeEvent;
    const cloudId = ctx.cloudId ?? 'unknown';
    const eventType = evt.eventType ?? 'unknown';
    const key = `${QUEUE_PREFIX}${cloudId}:${Date.now()}:${randomSuffix()}`;
    const spaceKey = evt.content?.space?.key ?? 'none';
    const contentID = evt.content?.id ?? 'none';
    console.log(`enqueue: type=${eventType} cloudId=${cloudId} spaceKey=${spaceKey} contentID=${contentID} key=${key}`);

    const enriched = await enrichWithBody(evt);
    const safe = enforceStorageLimit(enriched, contentID);

    try {
        await storage.set(key, { event: safe, context, enqueuedAt: Date.now() });
        console.log(`enqueue: stored key=${key} bodyAttached=${Boolean(safe.content?.body)}`);
    } catch (err) {
        console.error(`enqueue: storage.set failed key=${key} error=${(err as Error)?.message ?? err}`);
        throw err;
    }
};

const enforceStorageLimit = (evt: ForgeEvent, contentID: string | number): ForgeEvent => {
    if (!evt.content?.body) return evt;
    const budget = FORGE_STORAGE_MAX_BYTES - FORGE_STORAGE_ENVELOPE_HEADROOM;
    const size = byteLength(JSON.stringify(evt));
    if (size <= budget) return evt;
    console.log(`enqueue: body too large for Forge storage (size=${size} budget=${budget} contentId=${contentID}); dropping body, mentions will be skipped for this event`);
    const {body: _body, ...restContent} = evt.content;
    return { ...evt, content: restContent };
};

const byteLength = (s: string): number => {
    // Node 18+ on Forge runtime exposes TextEncoder globally.
    return new TextEncoder().encode(s).length;
};

const enrichWithBody = async (evt: ForgeEvent): Promise<ForgeEvent> => {
    const contentID = evt.content?.id != null ? String(evt.content.id) : '';
    const contentType = evt.content?.type ?? '';
    if (!contentID) return evt;

    let url: ReturnType<typeof route> | null = null;
    if (contentType === 'page') {
        url = route`/wiki/api/v2/pages/${contentID}?body-format=atlas_doc_format`;
    } else if (contentType === 'comment') {
        const location = evt.content?.extensions?.location;
        url = location === 'inline'
            ? route`/wiki/api/v2/inline-comments/${contentID}?body-format=atlas_doc_format`
            : route`/wiki/api/v2/footer-comments/${contentID}?body-format=atlas_doc_format`;
    }
    if (!url) return evt;

    try {
        const resp = await api.asApp().requestConfluence(url, { headers: { Accept: 'application/json' } });
        if (!resp.ok) {
            console.log(`enqueue: body fetch failed status=${resp.status} contentId=${contentID} type=${contentType}`);
            return evt;
        }
        const data = await resp.json() as { body?: { atlas_doc_format?: { value?: string } } };
        const body = data.body?.atlas_doc_format?.value;
        if (!body) return evt;
        return {
            ...evt,
            content: { ...(evt.content ?? {}), body },
        };
    } catch (err) {
        console.log(`enqueue: body fetch threw contentId=${contentID} type=${contentType} error=${(err as Error)?.message ?? err}`);
        return evt;
    }
};

// drain returns up to MAX_DRAIN_BATCH queued events and deletes the cursor of
// keys the caller acks. The plugin authenticates by HMAC-signing the request
// body with the shared secret set via the `register` trigger.
export const drain = async (req: WebTriggerRequest): Promise<WebTriggerResponse> => {
    console.log('drain: invoked');
    const secret = (await storage.getSecret(SECRET_KEY)) as string | undefined;
    if (!secret) {
        console.log('drain: rejected, bridge not registered');
        return jsonResponse(503, { error: 'bridge not registered; POST credentials to register web trigger first' });
    }

    if (!verifySignature(secret, headerValue(req, 'x-mm-signature'), req.body ?? '')) {
        console.log('drain: rejected, invalid signature');
        return jsonResponse(403, { error: 'invalid signature' });
    }

    let body: DrainRequest = {};
    if (req.body) {
        try {
            body = JSON.parse(req.body);
        } catch {
            return jsonResponse(400, { error: 'invalid JSON body' });
        }
    }

    if (body.ack?.length) {
        const ackable = body.ack.filter((k) => typeof k === 'string' && k.startsWith(QUEUE_PREFIX));
        await Promise.all(ackable.map((k) => storage.delete(k)));
        console.log(`drain: acked ${ackable.length} keys`);
    }

    const limit = clampLimit(body.limit);
    const results = await storage
        .query()
        .where('key', { condition: 'STARTS_WITH', value: QUEUE_PREFIX })
        .limit(limit)
        .getMany();

    const events = results.results.map((r) => ({ key: r.key, value: r.value }));
    console.log(`drain: returning ${events.length} events (limit=${limit})`);
    return jsonResponse(200, { events, nextCursor: results.nextCursor ?? null });
};

// register is a one-shot. Once `mm.registered` is set, further calls are
// refused. To re-register, an operator must clear the flag via the Forge CLI:
//   forge install --upgrade   then re-POST to register
export const register = async (req: WebTriggerRequest): Promise<WebTriggerResponse> => {
    let payload: { secret?: string; force?: boolean };
    try {
        payload = JSON.parse(req.body ?? '{}');
    } catch {
        return jsonResponse(400, { error: 'invalid JSON body' });
    }

    if (!payload.secret || payload.secret.length < 32) {
        return jsonResponse(400, { error: 'secret must be at least 32 characters' });
    }

    if ((await storage.get(REGISTERED_KEY)) && !payload.force) {
        return jsonResponse(409, { error: 'already registered; re-POST with {"force": true} to overwrite' });
    }

    await storage.setSecret(SECRET_KEY, payload.secret);
    await storage.set(REGISTERED_KEY, true);
    return jsonResponse(200, { ok: true });
};

export const onInstalled = async (): Promise<void> => {
    try {
        const registerURL = await webTrigger.getUrl('register');
        const drainURL = await webTrigger.getUrl('drain');
        console.log(`forge:installed register URL = ${registerURL}`);
        console.log(`forge:installed drain URL    = ${drainURL}`);
        console.log(
            'forge:installed Paste the drain URL into Mattermost System Console > Confluence > Forge Drain URL, ' +
                'then POST {"secret":"<32+ chars>"} to the register URL using the matching shared secret.',
        );
    } catch (err) {
        console.error(`forge:installed failed to resolve web trigger URLs: ${err}`);
    }
};

const verifySignature = (secret: string, providedHex: string | undefined, body: string): boolean => {
    if (!providedHex) return false;
    const expected = createHmac('sha256', secret).update(body).digest();
    let provided: Buffer;
    try {
        provided = Buffer.from(providedHex, 'hex');
    } catch {
        return false;
    }
    if (provided.length !== expected.length) return false;
    return timingSafeEqual(provided, expected);
};

const headerValue = (req: WebTriggerRequest, name: string): string | undefined => {
    const headers = req.headers ?? {};
    for (const key of Object.keys(headers)) {
        if (key.toLowerCase() === name.toLowerCase()) {
            const v = headers[key];
            return Array.isArray(v) ? v[0] : v;
        }
    }
    return undefined;
};

const randomSuffix = (): string => Math.random().toString(36).slice(2, 10);

const clampLimit = (raw: unknown): number => {
    const n = typeof raw === 'number' ? raw : Number(raw);
    if (!Number.isFinite(n) || n <= 0) return MAX_DRAIN_BATCH;
    return Math.min(Math.max(Math.floor(n), 1), MAX_DRAIN_BATCH);
};

type DrainRequest = { limit?: number; ack?: string[] };

type WebTriggerRequest = {
    body?: string;
    headers?: Record<string, string[] | string>;
    method: string;
    queryParameters?: Record<string, string[] | string>;
};

type WebTriggerResponse = {
    statusCode: number;
    headers?: Record<string, string[]>;
    body: string;
};

const jsonResponse = (status: number, payload: unknown): WebTriggerResponse => ({
    statusCode: status,
    headers: { 'Content-Type': ['application/json'] },
    body: JSON.stringify(payload),
});
