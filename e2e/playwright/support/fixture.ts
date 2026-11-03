// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {Channel} from '@mattermost/types/channels';
import type {Team} from '@mattermost/types/teams';
import type {UserProfile} from '@mattermost/types/users';
import {expect, test as base} from '@mattermost/playwright-lib';

import {cloudId, defaultUser} from '../fake-confluence/state';

import {confluenceFlavor} from './constants';
import {fakeConfluence} from './fake_confluence';
import {seedConnection, type SeededUser} from './kv';

export {expect};

export type ConfluenceFixture = {
    adminClient: Client4;
    user: UserProfile;
    userClient: Client4;
    team: Team;
    channel: Channel;
    connect: (mattermostUser?: UserProfile) => SeededUser;
};

export type FailureScreenshotOptions = {failureScreenshots: boolean};

// pw.initSetup() is deliberately avoided: it resets server config, which drops the plugin's settings and PluginStates.
export const test = base.extend<{confluence: ConfluenceFixture; failureScreenshot: void}, FailureScreenshotOptions>({
    failureScreenshots: [false, {option: true, scope: 'worker'}],

    // Playwright's own failure screenshot runs after pw closes the context and only sees a blank page.
    failureScreenshot: [async ({pw, failureScreenshots}, use, testInfo) => {
        await use();
        if (!failureScreenshots || testInfo.status === testInfo.expectedStatus) {
            return;
        }
        for (const [i, page] of (pw.testBrowser.context?.pages() ?? []).entries()) {
            const body = await page.screenshot({fullPage: true}).catch(() => undefined);
            if (body) {
                await testInfo.attach(`failure-page-${i}`, {body, contentType: 'image/png'});
            }
        }
    }, {auto: true}],

    confluence: async ({pw}, use) => {
        await fakeConfluence.reset();

        const {adminClient} = await pw.getAdminClient();
        const team = await pw.createNewTeam(adminClient, {name: 'confluence', displayName: 'Confluence', type: 'O', unique: true});
        const user = await pw.createNewUserProfile(adminClient, {prefix: 'confluence', disableTutorial: true, disableOnboarding: true});
        await adminClient.addToTeam(team.id, user.id);
        const {client: userClient} = await pw.makeClient(user);
        if (!userClient) {
            throw new Error(`Failed to log in ${user.username}`);
        }
        const channel = await adminClient.getChannelByName(team.id, 'town-square');

        const connect = (mattermostUser: UserProfile = user): SeededUser => {
            const confluenceUser = defaultUser();
            const seeded: SeededUser = {
                accountId: confluenceFlavor === 'cloud' ? confluenceUser.accountId : confluenceUser.userKey,
                username: confluenceUser.username,
                displayName: confluenceUser.displayName,
                cloudId: confluenceFlavor === 'cloud' ? cloudId : undefined,
            };
            seedConnection(mattermostUser.id, seeded);
            return seeded;
        };

        await use({adminClient, user, userClient, team, channel, connect});

        expect(await fakeConfluence.contractViolations(), 'fake-confluence API traffic must match Atlassian\'s OpenAPI spec').toEqual([]);
    },
});
