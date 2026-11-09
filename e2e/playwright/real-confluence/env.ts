// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import path from 'node:path';

import '../support/constants';

export type ConfluenceAccount = {email: string; password?: string; totpSecret?: string};

const required = [
    'CONFLUENCE_CLOUD_URL',
    'CONFLUENCE_SPACE_KEY',
    'CONFLUENCE_CLOUD_USER_EMAIL',
    'CONFLUENCE_CLOUD_OAUTH_CLIENT_ID',
    'CONFLUENCE_CLOUD_OAUTH_CLIENT_SECRET',
] as const;

export function missingRealConfluenceEnv() {
    return required.filter((name) => !process.env[name]);
}

const env = (name: (typeof required)[number]) => process.env[name] ?? '';

export const realConfluence = {
    siteURL: env('CONFLUENCE_CLOUD_URL').replace(/\/$/, ''),
    spaceKey: env('CONFLUENCE_SPACE_KEY'),
    user: {
        email: env('CONFLUENCE_CLOUD_USER_EMAIL'),
        password: process.env.CONFLUENCE_CLOUD_USER_PASSWORD || undefined,
        totpSecret: process.env.CONFLUENCE_CLOUD_USER_TOTP_SECRET || undefined,
    } as ConfluenceAccount,
    oauth: {clientId: env('CONFLUENCE_CLOUD_OAUTH_CLIENT_ID'), clientSecret: env('CONFLUENCE_CLOUD_OAUTH_CLIENT_SECRET')},
    userSessionPath: path.join(__dirname, '../.auth/atlassian-user.json'),
};
