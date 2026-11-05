// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {randomUUID} from 'node:crypto';

import {Client4} from '@mattermost/client';
import type {Channel} from '@mattermost/types/channels';
import type {Team} from '@mattermost/types/teams';
import type {UserProfile} from '@mattermost/types/users';
import {expect, test} from '@mattermost/playwright-lib';

import {loginAdmin} from '../support/admin';
import {mattermostURL, pluginId} from '../support/constants';
import {openChannel, runSlashCommand} from '../support/ui';

import {realConfluence} from './env';
import {addAtlassianSession, approveAtlassianConsent} from './login/session';

test.describe.configure({mode: 'serial'});

const password = 'passwd';

let team: Team;
let channel: Channel;
let user: UserProfile;
let userClient: Client4;

test.beforeAll(async () => {
    const admin = await loginAdmin();
    const id = randomUUID().slice(0, 8);
    team = await admin.createTeam({name: `confluence-real-${id}`, display_name: `Confluence real ${id}`, type: 'O'} as Team);
    user = await admin.createUser({username: `confluence-real-${id}`, email: `confluence-real-${id}@example.com`, password} as UserProfile, '', '');
    await admin.addToTeam(team.id, user.id);
    channel = await admin.getChannelByName(team.id, 'town-square');

    userClient = new Client4();
    userClient.setUrl(mattermostURL);
    await userClient.login(user.username, password);
});

/**
 * @objective Verify the regular user links their account to the real Confluence Cloud site through Atlassian's consent screen.
 *
 * @precondition
 * The Atlassian OAuth app has the plugin's callback URL for MM_PORT and the Confluence scopes; the user's Atlassian session is saved in .auth/.
 */
test('connects regular user to real Confluence Cloud through Atlassian consent', {tag: '@confluence_plugin'}, async ({pw}) => {
    // # Run /confluence connect with the saved Atlassian session
    const {channelsPage, page} = await openChannel(pw, {...user, password}, team.name);
    await addAtlassianSession(page.context());
    await runSlashCommand(channelsPage, '/confluence connect');

    // # Follow the connect link and approve the consent screen
    const popupPromise = page.context().waitForEvent('page');
    await page.getByRole('link', {name: 'Click here to link your Confluence account'}).click();
    const popup = await popupPromise;
    await approveAtlassianConsent(popup);

    // * Verify the flow ends on the connected page
    await expect(popup.getByRole('heading', {name: 'Mattermost user is now connected to Confluence'})).toBeVisible();
});

/**
 * @objective Verify the connected user subscribes the channel to a real Confluence space.
 *
 * @precondition
 * Runs after the connect test; CONFLUENCE_SPACE_KEY names a space the regular user can read.
 */
test('subscribes channel to a real Confluence space as the connected user', {tag: '@confluence_plugin'}, async () => {
    // # Save a space subscription through the plugin API
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/${channel.id}/subscription/space_subscription`, {
        method: 'POST',
        headers: {Authorization: `Bearer ${userClient.getToken()}`, 'Content-Type': 'application/json'},
        body: JSON.stringify({
            alias: 'e2e-space',
            baseURL: realConfluence.siteURL,
            spaceKey: realConfluence.spaceKey,
            events: ['page_created'],
            channelID: channel.id,
            subscriptionType: 'space_subscription',
        }),
    });

    // * Verify the plugin accepted it
    expect(res.ok, await res.text()).toBe(true);
});
