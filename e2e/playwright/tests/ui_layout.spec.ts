// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';

import {longPageTitle} from '../fake-confluence/state';

import {instanceURL, longPageId, spaceKey} from '../support/constants';
import {expect, test, type ConfluenceFixture} from '../support/fixture';
import {expectInside, expectNoOverlap, expectNothingClipped} from '../support/layout';
import {publishPage, subscribeToTeamSpace} from '../support/notifications';
import {openChannel, runSlashCommand} from '../support/ui';

async function openSubscribeModal(pw: PlaywrightExtended, confluence: ConfluenceFixture) {
    confluence.connect();
    const opened = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(opened.channelsPage, '/confluence subscribe');
    const dialog = opened.page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByTestId('subscription-name-input').fill('team-space');
    await dialog.getByTestId('subscription-url-input').fill(instanceURL);
    await dialog.getByTestId('subscription-space-key-input').fill(spaceKey);
    return {...opened, dialog};
}

/**
 * @objective Verify the subscription modal keeps its fields and buttons inside the dialog at wide and narrow widths.
 */
test('keeps subscription modal fields and actions inside the dialog at wide and narrow widths', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Open the subscription modal and fill it in
    const {page, dialog} = await openSubscribeModal(pw, confluence);
    const cancel = dialog.getByRole('button', {name: 'Cancel'});
    const save = dialog.getByRole('button', {name: 'Save Subscription'});

    for (const width of [1280, 900]) {
        // # Resize the window
        await page.setViewportSize({width, height: 900});
        await expect(save).toBeVisible();

        // * Verify the buttons don't overlap and nothing spills out of the dialog
        await expectNoOverlap(cancel, save);
        await expectInside(cancel, dialog);
        await expectInside(save, dialog);
        await expectInside(dialog.getByTestId('subscription-space-key-input'), dialog);
        await expectNothingClipped(dialog);
    }
});

/**
 * @objective Verify a notification for a page with a long title wraps the title inside the post.
 */
test('wraps long page title inside the notification post', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Publish a page with a long title in a subscribed space
    const seeded = confluence.connect();
    await subscribeToTeamSpace(confluence.userClient, confluence.channel.id);
    const created = await publishPage(confluence.userClient, confluence.channel.id, {title: longPageTitle, page: longPageId, userKey: seeded.accountId});

    // # Open the channel and expand the post if it is collapsed
    const {channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
    const post = await channelsPage.centerView.getPostById(created.id);
    const link = post.container.getByRole('link', {name: longPageTitle});

    // Server notifications render as message blocks, Cloud ones as markdown.
    const blocks = post.container.locator('.mm-blocks-text').filter({hasText: longPageTitle});
    const message = (await blocks.count()) > 0 ? blocks : post.container.locator('.post-message');
    const showMore = post.container.getByRole('button', {name: 'Show more'});
    if (await showMore.isVisible()) {
        await showMore.click();
    }

    // * Verify the title link stays inside the message and nothing is clipped
    await expect(link).toBeVisible();
    await expectInside(link, message);
    await expectNothingClipped(message);
});

/**
 * @objective Verify the subscription modal and a page notification have no axe accessibility violations.
 */
test('has no accessibility violations in the subscription modal and page notification', {tag: '@confluence_plugin'}, async ({pw, confluence, axe}) => {
    // # Publish a page, then open the subscription modal over the channel
    const seeded = confluence.connect();
    await subscribeToTeamSpace(confluence.userClient, confluence.channel.id, 'axe-space');
    const created = await publishPage(confluence.userClient, confluence.channel.id, {userKey: seeded.accountId});
    const {page, channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
    await runSlashCommand(channelsPage, '/confluence subscribe');
    await expect(page.getByRole('dialog')).toBeVisible();
    await (await channelsPage.centerView.getPostById(created.id)).toBeVisible();

    // Mid fade-in the modal text is translucent, which axe reports as low contrast.
    await page.evaluate(() => Promise.all(document.getAnimations().filter((a) => a.effect?.getTiming().iterations !== Infinity).map((a) => a.finished.catch(() => undefined))));

    // # Scan the modal and the notification
    // Compass field labels, tertiary buttons and combobox chips fail these two rules upstream.
    const modalResults = await axe.builder(page, {disableLinkInTextBlock: true}).include('[role="dialog"]').disableRules(['color-contrast', 'nested-interactive']).analyze();
    const postResults = await axe.builder(page, {disableLinkInTextBlock: true}).include(`#post_${created.id} .post__body`).analyze();

    // * Verify there are no violations
    const violations = [...modalResults.violations, ...postResults.violations];
    expect(violations.map((v) => `${v.id}: ${v.help} (${v.nodes.length})`)).toEqual([]);
});
