// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defaultUser} from '../fake-confluence/state';

import {confluenceFlavor, mattermostURL, pluginId} from '../support/constants';
import {fakeConfluence} from '../support/fake_confluence';
import {expect, test} from '../support/fixture';
import {publishPage, subscribeToTeamSpace} from '../support/notifications';

/**
 * @objective Verify the Data Center webhook endpoint rejects requests without the shared secret.
 *
 * @precondition
 * Runs only with CONFLUENCE_FLAVOR=server; Cloud events arrive through the Forge drain.
 */
test('rejects server webhook without the shared secret', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async () => {
    test.skip(confluenceFlavor !== 'server', 'Cloud events arrive through the Forge drain');

    // # Post a page created event without the secret
    const res = await fetch(`${mattermostURL}/plugins/${pluginId}/api/v1/server/webhook`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({event: 'page_created', userKey: defaultUser().userKey, page: {id: 1001}}),
    });

    // * Verify the plugin refuses it
    expect(res.status).toBe(403);
});

/**
 * @objective Verify the Forge drain endpoint accepts an HMAC-signed request and rejects a bad signature.
 */
test('accepts signed Forge drain request and rejects bad signature', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async () => {
    // # Send a correctly signed drain request
    const ok = await fakeConfluence.postDrain({limit: 0});

    // * Verify it is accepted
    expect(ok.status).toBe(200);

    // # Send a drain request with a bad signature
    const bad = await fakeConfluence.postDrain({limit: 0}, 'not-a-signature');

    // * Verify it is rejected
    expect(bad.status).toBe(401);
});

/**
 * @objective Verify a page created in a subscribed space posts a notification to the channel.
 */
test('notifies subscribed channel when a page is created', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to the space and publish a page
    const seeded = confluence.connect();
    await subscribeToTeamSpace(confluence.userClient, confluence.channel.id);
    const post = await publishPage(confluence.userClient, confluence.channel.id, {userKey: seeded.accountId});

    // * Verify the notification names the page
    expect(post.message + JSON.stringify(post.props ?? {})).toContain('Release notes');
});
