// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Post} from '@mattermost/types/posts';
import type {Locator} from '@playwright/test';

import {commentCreatedEvent, longPageTitle, pageEvent, type ForgeEvent} from '../fake-confluence/state';

import {instanceURL, longPageId, pageId, spaceKey} from '../support/constants';
import {fakeConfluence} from '../support/fake_confluence';
import {expect, test, type ConfluenceFixture} from '../support/fixture';
import {publishPage, subscribe, subscribeToTeamSpace} from '../support/notifications';
import {openChannel, runSlashCommand, waitForPostText} from '../support/ui';

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

    async function openPost(pw: PlaywrightExtended, confluence: ConfluenceFixture, post: Post) {
        const {page, channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
        const message = page.locator(`#post_${post.id} .post-message`);
        await expect(message).toBeVisible();
        return {page, channelsPage, post, message};
    }

    async function pageNotification(pw: PlaywrightExtended, confluence: ConfluenceFixture, options: {title?: string; page?: string; alias?: string} = {}) {
        const seeded = confluence.connect();
        await subscribeToTeamSpace(confluence.userClient, confluence.channel.id, options.alias);
        const post = await publishPage(confluence.userClient, confluence.channel.id, {title: options.title, page: options.page, userKey: seeded.accountId});
        return openPost(pw, confluence, post);
    }

    async function forgeNotification(pw: PlaywrightExtended, confluence: ConfluenceFixture, event: ForgeEvent) {
        confluence.connect();
        await subscribeToTeamSpace(confluence.userClient, confluence.channel.id);
        await fakeConfluence.enqueueForgeEvent(event);
        return openPost(pw, confluence, await waitForPostText(confluence.userClient, confluence.channel.id, 'Release notes'));
    }

    async function openSubscribeModal(pw: PlaywrightExtended, confluence: ConfluenceFixture, command = '/confluence subscribe') {
        const {channelsPage, page} = await openChannel(pw, confluence.user, confluence.team.name);
        await runSlashCommand(channelsPage, command);
        const modal = page.getByRole('dialog');
        await expect(modal).toBeVisible();
        return {page, modal};
    }

    async function slashCommandReply(pw: PlaywrightExtended, confluence: ConfluenceFixture, command: string, text: string) {
        const {channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
        await runSlashCommand(channelsPage, command);
        const reply = await channelsPage.centerView.getLastPost();
        await reply.toContainText(text);
        return reply.container.locator('.post-message');
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
        const {modal} = await openSubscribeModal(pw, confluence);
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

    /**
     * @objective Verify the comment created notification renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for comment created notification', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({pw, confluence}) => {
        // # Comment on a page in the subscribed space
        const {message} = await forgeNotification(pw, confluence, commentCreatedEvent());
        await settle(pw, message);

        // * Verify the post matches the baseline
        await expect(message).toHaveScreenshot('comment-created.png');
    });

    /**
     * @objective Verify the page updated notification renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for page updated notification', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({pw, confluence}) => {
        // # Update a page in the subscribed space
        const {message} = await forgeNotification(pw, confluence, pageEvent('updated'));
        await settle(pw, message);

        // * Verify the post matches the baseline
        await expect(message).toHaveScreenshot('page-updated.png');
    });

    /**
     * @objective Verify the page trashed notification renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for page trashed notification', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({pw, confluence}) => {
        // # Trash a page in the subscribed space
        const {message} = await forgeNotification(pw, confluence, pageEvent('trashed'));
        await settle(pw, message);

        // * Verify the post matches the baseline
        await expect(message).toHaveScreenshot('page-trashed.png');
    });

    /**
     * @objective Verify the subscription modal renders like the baseline when set up for a single page.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for page subscription modal', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Open the subscription modal, switch it to a page subscription and fill it in
        confluence.connect();
        const {page, modal} = await openSubscribeModal(pw, confluence);
        await modal.getByRole('combobox', {name: 'Subscribe To'}).press('ArrowDown');
        await page.getByRole('option', {name: 'Page', exact: true}).click();
        await modal.getByTestId('subscription-name-input').fill('release-page');
        await modal.getByTestId('subscription-url-input').fill(instanceURL);
        await modal.getByTestId('subscription-page-id-input').fill(pageId);
        await settle(pw, modal);

        // * Verify the modal matches the baseline
        await expect(modal).toHaveScreenshot('subscription-modal-page.png');
    });

    /**
     * @objective Verify the subscription modal's required field errors render like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for subscription modal validation errors', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Save the subscription modal with its text fields empty
        confluence.connect();
        const {modal} = await openSubscribeModal(pw, confluence);
        await modal.getByRole('button', {name: 'Save Subscription'}).click();
        await expect(modal.getByText('This field is required.')).toHaveCount(3);
        await settle(pw, modal);

        // * Verify the modal matches the baseline
        await expect(modal).toHaveScreenshot('subscription-modal-errors.png');
    });

    /**
     * @objective Verify the edit subscription modal renders like the baseline with the saved values loaded.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for edit subscription modal', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Save a subscription with two events and open it with /confluence edit
        confluence.connect();
        await subscribe(confluence.userClient, confluence.channel.id, {events: ['comment_created', 'page_created']});
        const {modal} = await openSubscribeModal(pw, confluence, '/confluence edit team-space');
        await expect(modal.getByTestId('subscription-name-input')).toHaveValue('team-space');
        await settle(pw, modal);

        // * Verify the modal matches the baseline
        await expect(modal).toHaveScreenshot('subscription-modal-edit.png');
    });

    /**
     * @objective Verify the /confluence list reply renders its subscription tables like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for subscription list reply', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Save a space and a page subscription and list them
        confluence.connect();
        await subscribe(confluence.userClient, confluence.channel.id, {events: ['page_created', 'page_updated']});
        await subscribe(confluence.userClient, confluence.channel.id, {alias: 'release-page', page: pageId, events: ['comment_created']});
        const message = await slashCommandReply(pw, confluence, '/confluence list', 'Page Subscriptions');
        await settle(pw, message);

        // * Verify the reply matches the baseline
        await expect(message).toHaveScreenshot('subscription-list.png');
    });

    /**
     * @objective Verify the /confluence connect reply for a user who has not connected renders like the baseline.
     *
     * @precondition
     * Runs in the pinned dockerized browser and server (npm run test:visual).
     */
    test('matches baseline for connect prompt', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
        // # Run /confluence connect without connecting first
        const message = await slashCommandReply(pw, confluence, '/confluence connect', 'Click here to link your Confluence account');
        await settle(pw, message);

        // * Verify the reply matches the baseline
        await expect(message).toHaveScreenshot('connect-prompt.png');
    });
});
