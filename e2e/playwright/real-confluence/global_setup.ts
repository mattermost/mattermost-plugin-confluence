// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {baseGlobalSetup} from '@mattermost/playwright-lib';

import {loginAdmin} from '../support/admin';
import {installPlugin} from '../support/plugin';

import {missingRealConfluenceEnv, realConfluence} from './env';
import {ensureAtlassianSession} from './login/session';

export default async function globalSetup() {
    const missing = missingRealConfluenceEnv();
    if (missing.length) {
        throw new Error(`Set ${missing.join(', ')} in e2e/playwright/.env (see .env.example). Do not paste the values into chat.`);
    }
    await ensureAtlassianSession();
    await baseGlobalSetup();

    const adminClient = await loginAdmin();
    await installPlugin(adminClient, {
        confluenceurl: realConfluence.siteURL,
        confluenceoauthclientid: realConfluence.oauth.clientId,
        confluenceoauthclientsecret: realConfluence.oauth.clientSecret,
        iscloud: true,
        serverversiongreaterthan9: false,
        forgedrainurl: '',
    });
}
