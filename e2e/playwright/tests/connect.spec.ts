// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../support/fixture';
import {openChannel, runSlashCommand} from '../support/ui';

/**
 * @objective Verify a user links their Confluence account through the OAuth flow started by /confluence connect.
 */
test('connects Confluence account through OAuth from the slash command', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Run /confluence connect
    const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence connect');

    // # Follow the connect link, which opens the OAuth flow in a new window
    const popupPromise = page.context().waitForEvent('page');
    await page.getByRole('link', {name: 'Click here to link your Confluence account'}).click();
    const popup = await popupPromise;

    // * Verify the flow ends on the connected page with the Confluence user's name
    await expect(popup.getByRole('heading', {name: 'Mattermost user is now connected to Confluence'})).toBeVisible();
    await expect(popup.getByText('E2E User')).toBeVisible();
});
