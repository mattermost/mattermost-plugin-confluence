// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Locator} from '@playwright/test';

import {longPageTitle} from '../fake-confluence/state';

import {instanceURL, longPageId, spaceKey} from '../support/constants';
import {expect, test, type ConfluenceFixture} from '../support/fixture';
import {publishPage, subscribeToTeamSpace} from '../support/notifications';
import {openChannel, runSlashCommand} from '../support/ui';

test.describe('visual', {tag: '@visual'}, () => {
    // eslint-disable-next-line no-process-env
    test.skip(!process.env.PW_VISUAL_BROWSER_WS, 'baselines only match the dockerized browser (npm run test:visual)');

    // Core UI around the target (avatars, status, timestamps) changes between runs.
    async function settle(pw: PlaywrightExtended, target: Locator) {
        const page = target.page();
        await expect(page.locator('.LoadingScreen')).toBeHidden();
        await pw.hideDynamicChannelsContent(page);
        await page.mouse.move(0, 0);
        await page.evaluate(() => document.fonts.ready);

        // A modal measured mid slide-in gets snapped to the wrong pixel.
        await page.evaluate(() => Promise.all(document.getAnimations().
            filter((a) => a.effect?.getTiming().iterations !== Infinity).
            map((a) => a.finished.catch(() => undefined))));

        // Bottom-anchored posts and centred modals can sit at a fractional y, which moves text by a pixel row.
        await target.evaluate((el: HTMLElement) => {
            const {x, y} = el.getBoundingClientRect();
            el.style.transform = `translate(${Math.round(x) - x}px, ${Math.round(y) - y}px)`;
        });
    }

    async function pageNotification(pw: PlaywrightExtended, confluence: ConfluenceFixture, options: {title?: string; page?: string; alias?: string} = {}) {
        const seeded = confluence.connect();
        await subscribeToTeamSpace(confluence.userClient, confluence.channel.id, options.alias);
        const post = await publishPage(confluence.userClient, confluence.channel.id, {title: options.title, page: options.page, userKey: seeded.accountId});
        const {page, channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
        const message = page.locator(`#post_${post.id} .post-message`);
        await expect(message).toBeVisible();
        return {page, channelsPage, post, message};
    }

    /**
     * @objective Verify the subscription modal renders like the baseline with its fields filled in.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for subscription modal', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Open the subscription modal and fill in a space subscription
        confluence.connect();
        const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
        await runSlashCommand(channelsPage, '/confluence subscribe');
        const modal = page.locator('.modal-content');
        await expect(modal).toBeVisible();
        await modal.getByTestId('subscription-name-input').fill('team-space');
        await modal.getByTestId('subscription-url-input').fill(instanceURL);
        await modal.getByTestId('subscription-space-key-input').fill(spaceKey);
        await settle(pw, modal);

        // * Verify the modal matches the baseline
        await expect(modal).toHaveScreenshot('subscription-modal.png');
    });

    /**
     * @objective Verify the page created notification renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for page created notification', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Publish a page in the subscribed space
        const {message} = await pageNotification(pw, confluence);
        await settle(pw, message);

        // * Verify the post matches the baseline
        await expect(message).toHaveScreenshot('page-created.png');
    });

    /**
     * @objective Verify a notification with a long page title wraps like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for long page title notification', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Publish a page with a long title
        const {message} = await pageNotification(pw, confluence, {title: longPageTitle, page: longPageId, alias: 'long-title'});
        await expect(message.getByText(longPageTitle)).toBeVisible();
        await settle(pw, message);

        // * Verify the post matches the baseline
        await expect(message).toHaveScreenshot('long-title.png');
    });

    /**
     * @objective Verify a page notification in the reply thread renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for page notification in reply thread', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Open the notification's reply thread
        const {channelsPage, post} = await pageNotification(pw, confluence, {alias: 'thread'});
        await (await channelsPage.centerView.getPostById(post.id)).reply();
        const rhsPost = await channelsPage.sidebarRight.getPostById(post.id);
        const message = rhsPost.container.locator('.post-message');
        await expect(message).toBeVisible();
        await settle(pw, message);

        // * Verify the thread post matches the baseline
        await expect(message).toHaveScreenshot('reply-thread.png');
    });
});
