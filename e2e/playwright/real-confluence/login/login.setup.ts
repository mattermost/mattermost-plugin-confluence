// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-console */

import {test} from '@playwright/test';

import {realConfluence} from '../env';

import {saveSession, signInToAtlassian} from './session';

// Forces a fresh Atlassian session; `make e2e-real` also refreshes it by itself once it's 3 days old.
test('sign in to Atlassian as the regular test user', async ({page}) => {
    test.setTimeout(3 * 60_000);
    await signInToAtlassian(page, 2 * 60_000);
    await saveSession(page);
    console.log(`Saved the Atlassian session to ${realConfluence.userSessionPath}`);
});
