import { storage, webTrigger } from '@forge/api';
import { createHmac, timingSafeEqual } from 'crypto';

const QUEUE_PREFIX = 'evt:';
const SECRET_KEY = 'mm.drainSecret';
const REGISTERED_KEY = 'mm.registered';
const MAX_DRAIN_BATCH = 100;

export const enqueue = async (event: unknown, context: unknown): Promise<void> => {
    const cloudId = (context as { cloudId?: string })?.cloudId ?? 'unknown';
    const key = `${QUEUE_PREFIX}${cloudId}:${Date.now()}:${randomSuffix()}`;
    await storage.set(key, { event, context, enqueuedAt: Date.now() });
};

// drain returns up to MAX_DRAIN_BATCH queued events and deletes the cursor of
// keys the caller acks. The plugin authenticates by HMAC-signing the request
// body with the shared secret set via the `register` trigger.
export const drain = async (req: WebTriggerRequest): Promise<WebTriggerResponse> => {
    const secret = (await storage.getSecret(SECRET_KEY)) as string | undefined;
    if (!secret) {
        return jsonResponse(503, { error: 'bridge not registered; POST credentials to register web trigger first' });
    }

    if (!verifySignature(secret, headerValue(req, 'x-mm-signature'), req.body ?? '')) {
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
    }

    const limit = clampLimit(body.limit);
    const results = await storage
        .query()
        .where('key', { condition: 'STARTS_WITH', value: QUEUE_PREFIX })
        .limit(limit)
        .getMany();

    const events = results.results.map((r) => ({ key: r.key, value: r.value }));
    return jsonResponse(200, { events, nextCursor: results.nextCursor ?? null });
};

// register is a one-shot. Once `mm.registered` is set, further calls are
// refused. To re-register, an operator must clear the flag via the Forge CLI:
//   forge install --upgrade   then re-POST to register
export const register = async (req: WebTriggerRequest): Promise<WebTriggerResponse> => {
    if (await storage.get(REGISTERED_KEY)) {
        return jsonResponse(409, { error: 'already registered; clear mm.registered to re-register' });
    }

    let payload: { secret?: string };
    try {
        payload = JSON.parse(req.body ?? '{}');
    } catch {
        return jsonResponse(400, { error: 'invalid JSON body' });
    }

    if (!payload.secret || payload.secret.length < 32) {
        return jsonResponse(400, { error: 'secret must be at least 32 characters' });
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
    return Math.min(Math.floor(n), MAX_DRAIN_BATCH);
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
