// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-process-env */

import {defineConfig, devices} from '@playwright/test';

import {mattermostURL} from './support/constants';
import type {FailureScreenshotOptions} from './support/fixture';

const isCI = Boolean(process.env.CI);

// Full journey against a real Confluence Cloud test site. Needs a normal `make dist` build, not `make e2e-dist`.
export default defineConfig<FailureScreenshotOptions>({
    testDir: './real-confluence',
    testMatch: /journey\.spec\.ts$/,
    outputDir: './test-results/real-confluence',
    globalSetup: require.resolve('./real-confluence/global_setup'),
    fullyParallel: false,
    workers: 1,
    forbidOnly: isCI,
    retries: 0,
    timeout: 120_000,
    expect: {timeout: 30_000},
    reporter: [['list'], ['html', {open: 'never', outputFolder: 'playwright-report/real-confluence'}]],
    use: {
        baseURL: mattermostURL,
        headless: isCI || process.env.PW_HEADLESS === 'true',
        launchOptions: {slowMo: Number(process.env.PW_SLOWMO ?? 0)},

        // Real tokens and Atlassian cookies end up in traces, so they stay off in CI.
        trace: isCI ? 'off' : 'retain-on-failure',
        screenshot: 'off',
        video: 'off',
        failureScreenshots: !isCI,
    },
    projects: [
        {
            name: 'real-confluence',
            use: {...devices['Desktop Chrome'], viewport: {width: 1280, height: 1024}},
        },
    ],
});
