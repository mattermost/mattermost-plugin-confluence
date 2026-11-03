// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {instanceURL, spaceKey} from '../support/constants';
import {expect, test} from '../support/fixture';
import {openChannel, runSlashCommand} from '../support/ui';

/**
 * @objective Verify a connected user saves a space subscription from the /confluence subscribe modal.
 */
test('saves space subscription from the subscribe modal', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Open the subscribe modal as a connected user
    confluence.connect();
    const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence subscribe');
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    // # Fill in a space subscription and save it
    await dialog.getByTestId('subscription-name-input').fill('team-space');
    await dialog.getByTestId('subscription-url-input').fill(instanceURL);
    await dialog.getByTestId('subscription-space-key-input').fill(spaceKey);
    await dialog.getByRole('button', {name: 'Save Subscription'}).click();

    // * Verify the save is confirmed
    await expect(page.getByText('Your subscription has been saved.', {exact: true})).toBeVisible();
});
