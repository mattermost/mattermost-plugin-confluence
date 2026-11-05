// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import {defineConfig} from '@playwright/test';

import './support/constants';

// Talks only to Atlassian. No Mattermost, no traces.
export default defineConfig({
    testDir: './real-confluence/login',
    testMatch: /login\.setup\.ts$/,
    outputDir: './test-results/real-login',
    workers: 1,
    timeout: 30_000,
    reporter: [['list']],
    use: {
        trace: 'off',
        screenshot: 'off',
        video: 'off',
    },
});
