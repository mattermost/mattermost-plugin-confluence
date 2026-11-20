// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {pageEvent} from '../fake-confluence/state';

import {confluenceFlavor, instanceURL, pageId, spaceKey} from '../support/constants';
import {expect, test} from '../support/fixture';
import {channelPostTexts, deliverForgeEvent, subscribe} from '../support/notifications';
import {openChannel, runSlashCommand, waitForPostText} from '../support/ui';

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

/**
 * @objective Verify an event removed from a subscription in the edit modal is no longer posted to the channel.
 */
test('stops notifying for an event removed in the edit modal', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({pw, confluence}) => {
    test.skip(confluenceFlavor !== 'cloud', 'Forge events are Cloud only');

    // # Subscribe the channel to every event and open the edit modal
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id);
    const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence edit team-space');
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByTestId('subscription-name-input')).toHaveValue('team-space');

    // # Remove Page Update from the events and save
    await dialog.getByRole('button', {name: 'Page Update. Press Delete to remove.'}).press('Delete');
    await dialog.getByRole('button', {name: 'Save Subscription'}).click();

    // * Verify the edit is confirmed
    await expect(page.getByText('Your subscription has been edited successfully.', {exact: true})).toBeVisible();

    // # Update a page, then create another one
    await deliverForgeEvent(pageEvent('updated'));
    await deliverForgeEvent(pageEvent('created', 'Launch plan'));

    // * Verify only the page created notification is posted
    await waitForPostText(confluence.userClient, confluence.channel.id, 'Launch plan');
    expect((await channelPostTexts(confluence.userClient, confluence.channel.id)).filter((text) => text.includes('was updated'))).toEqual([]);
});

/**
 * @objective Verify /confluence unsubscribe deletes the channel's subscription.
 */
test('deletes subscription with /confluence unsubscribe', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Subscribe the channel and unsubscribe it with the slash command
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id);
    const {channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence unsubscribe team-space');

    // * Verify the deletion is confirmed
    await (await channelsPage.centerView.getLastPost()).toContainText('Subscription team-space has been deleted.');

    // # List the channel's subscriptions
    await runSlashCommand(channelsPage, '/confluence list');

    // * Verify none are left
    await (await channelsPage.centerView.getLastPost()).toContainText('No subscriptions found for this channel.');
});

/**
 * @objective Verify /confluence list shows the channel's space and page subscriptions.
 */
test('lists space and page subscriptions with /confluence list', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Subscribe the channel to a space and to a page, then list its subscriptions
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id, {events: ['page_created']});
    await subscribe(confluence.userClient, confluence.channel.id, {alias: 'release-page', page: pageId, events: ['comment_created']});
    const {channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence list');

    // * Verify both subscriptions are listed with their target and events
    const list = (await channelsPage.centerView.getLastPost()).body;
    await expect(list.getByRole('heading', {name: 'Space Subscriptions'})).toBeVisible();
    await expect(list.getByRole('row', {name: `team-space ${instanceURL} ${spaceKey} Page Create`})).toBeVisible();
    await expect(list.getByRole('heading', {name: 'Page Subscriptions'})).toBeVisible();
    await expect(list.getByRole('row', {name: `release-page ${instanceURL} ${pageId} Comment Create`})).toBeVisible();
});

/**
 * @objective Verify a user who has not connected Confluence gets the connect prompt instead of the subscribe modal.
 */
test('prompts non-connected user to connect on /confluence subscribe', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Run /confluence subscribe without connecting
    const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence subscribe');

    // * Verify the connect prompt is posted and no modal opens
    await (await channelsPage.centerView.getLastPost()).toContainText('User not connected. Please use /confluence connect.');
    await expect(page.getByRole('dialog')).toHaveCount(0);
});
