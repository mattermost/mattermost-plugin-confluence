// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import {createHash} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

// docker compose reads the same file, so MM_PORT/MM_IMAGE stay in sync between the stack and the tests.
const envFile = path.join(__dirname, '../.env');
if (fs.existsSync(envFile)) {
    process.loadEnvFile(envFile);
}

export const pluginId = 'com.mattermost.confluence';
export const pluginDistDir = path.join(__dirname, '../../../dist');

// Must match the server's SiteURL; the plugin rejects "localhost" so 127.0.0.1 is used.
process.env.PW_BASE_URL ??= `http://127.0.0.1:${process.env.MM_PORT ?? 8065}`;
export const mattermostURL = process.env.PW_BASE_URL;

export type ConfluenceFlavor = 'cloud' | 'server';
export const confluenceFlavor: ConfluenceFlavor = process.env.CONFLUENCE_FLAVOR === 'server' ? 'server' : 'cloud';

// Fake hostnames resolve from the Mattermost container (extra_hosts) and from Chromium (--host-resolver-rules).
export const fakeConfluenceHost = 'fake-confluence';
export const fakeCloudSiteHost = 'e2e.atlassian.net';
export const fakeConfluencePort = Number(process.env.FAKE_CONFLUENCE_PORT ?? 8092);
export const fakeConfluenceControlURL = `http://127.0.0.1:${fakeConfluencePort}`;

export const fakeServerURL = `http://${fakeConfluenceHost}:${fakeConfluencePort}`;
export const fakeCloudSiteURL = `http://${fakeCloudSiteHost}:${fakeConfluencePort}`;
export const instanceURL = confluenceFlavor === 'cloud' ? fakeCloudSiteURL : fakeServerURL;
export const forgeDrainURL = `${fakeServerURL}/forge/drain`;

// Generated plugin settings must be exactly 32 characters or the plugin replaces them on activate.
export const webhookSecret = 'e2e-confluence-webhook-secret!!!';
export const encryptionKey = 'e2e-encryption-key-0123456789abc';

// Every running E2E stack polls the same fake-confluence host port. A per-stack secret makes the drain refuse the
// other stacks, which would otherwise drain and ack this run's events.
export const forgeSharedSecret = createHash('sha256').update(`forge:${mattermostURL}`).digest('hex').slice(0, 32);

export const oauthClientId = 'e2e-confluence-client-id';
export const oauthClientSecret = 'e2e-confluence-client-secret';

export const spaceKey = 'TEAM';
export const pageId = '1001';
export const longPageId = '1002';
