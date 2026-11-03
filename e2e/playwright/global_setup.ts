// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {baseGlobalSetup} from '@mattermost/playwright-lib';

import {startFakeConfluence} from './fake-confluence/server';
import {loginAdmin} from './support/admin';
import {fakeCloudSiteURL, fakeConfluencePort, forgeSharedSecret} from './support/constants';
import {installPlugin} from './support/plugin';

export default async function globalSetup() {
    await baseGlobalSetup();

    const fake = await startFakeConfluence({
        port: fakeConfluencePort,
        siteURL: fakeCloudSiteURL,
        forgeSecret: forgeSharedSecret,
    });

    const adminClient = await loginAdmin();
    await installPlugin(adminClient);

    return fake.close;
}
