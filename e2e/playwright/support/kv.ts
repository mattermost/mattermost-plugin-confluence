// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {execFileSync} from 'node:child_process';
import {createCipheriv, randomBytes} from 'node:crypto';
import path from 'node:path';

import {encryptionKey, instanceURL, pluginId} from './constants';

export type SeededUser = {accountId: string; username: string; displayName: string; isAdmin?: boolean; cloudId?: string};

const composeDir = path.join(__dirname, '..');

// Keys and ciphertext must match store.StoreConnection / store.StoreUser and auth_token.go (AES-GCM, nonce prefix).
export function seedConnection(mattermostUserId: string, user: SeededUser, accessToken = `acc-${mattermostUserId}`) {
    const connection = {
        accountId: user.accountId,
        username: user.username,
        displayName: user.displayName,
        token: encryptToken(accessToken),
        is_admin: Boolean(user.isAdmin),
        mattermost_user_id: mattermostUserId,
        cloud_id: user.cloudId ?? '',
    };
    upsert({
        [`${instanceURL}_${mattermostUserId}`]: connection,
        [`${instanceURL}_${user.accountId}`]: mattermostUserId,
        [`user_${mattermostUserId}`]: {mattermost_user_id: mattermostUserId, instance_url: instanceURL},
    });
}

function upsert(entries: Record<string, unknown>) {
    const values = Object.entries(entries).map(([key, value]) => `('${pluginId}', ${quote(key)}, convert_to(${quote(JSON.stringify(value))}, 'UTF8'), 0)`);
    psql(`INSERT INTO pluginkeyvaluestore (pluginid, pkey, pvalue, expireat) VALUES ${values.join(', ')}
        ON CONFLICT (pluginid, pkey) DO UPDATE SET pvalue = EXCLUDED.pvalue, expireat = 0;`);
}

function psql(sql: string) {
    execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'mmuser', '-d', 'mattermost_test', '-v', 'ON_ERROR_STOP=1', '-q'], {
        cwd: composeDir,
        input: sql,
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'inherit'],
    });
}

function quote(value: string) {
    return `'${value.replaceAll("'", "''")}'`;
}

// auth_token.go: AES-GCM, 12-byte nonce prefix, tag appended, base64 URL encoding with padding.
function encryptToken(accessToken: string) {
    const payload = JSON.stringify({
        token: {
            access_token: accessToken,
            token_type: 'Bearer',
            refresh_token: `ref-${accessToken}`,
            expiry: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
        },
    });
    const nonce = randomBytes(12);
    const cipher = createCipheriv('aes-256-gcm', Buffer.from(encryptionKey), nonce);
    const ciphertext = Buffer.concat([cipher.update(payload), cipher.final(), cipher.getAuthTag()]);
    return Buffer.concat([nonce, ciphertext]).toString('base64').replaceAll('+', '-').replaceAll('/', '_');
}
