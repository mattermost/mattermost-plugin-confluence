// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {commentCreatedEvent, longPageTitle, pageEvent, type ForgeEvent} from '../fake-confluence/state';

import {confluenceFlavor, instanceURL, longPageId, pageId, spaceKey} from '../support/constants';
import {fakeConfluence} from '../support/fake_confluence';
import {expect, test, type ConfluenceFixture} from '../support/fixture';
import {channelPostTexts, deliverForgeEvent, subscribe} from '../support/notifications';
import {waitForPostText} from '../support/ui';

test.skip(confluenceFlavor !== 'cloud', 'Forge events are Cloud only');

const pageURL = `${instanceURL}/wiki/spaces/${spaceKey}/pages/${pageId}`;

async function notifySubscribedChannel(confluence: ConfluenceFixture, event: ForgeEvent) {
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id);
    await fakeConfluence.enqueueForgeEvent(event);
    return waitForPostText(confluence.userClient, confluence.channel.id, 'Release notes');
}

/**
 * @objective Verify a comment created on a page in a subscribed space posts a notification linking the comment and the page.
 */
test('notifies subscribed channel when a comment is created', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to the space and comment on a page
    const post = await notifySubscribedChannel(confluence, commentCreatedEvent('Release notes', pageId, '3001'));

    // * Verify the notification links the comment and its page
    expect(post.message).toBe(`A new [comment](${pageURL}?focusedCommentId=3001) was posted on the [Release notes](${pageURL}) page.`);
});

/**
 * @objective Verify a page updated in a subscribed space posts a notification.
 */
test('notifies subscribed channel when a page is updated', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to the space and update a page
    const post = await notifySubscribedChannel(confluence, pageEvent('updated'));

    // * Verify the notification names the page and the space
    expect(post.message).toBe(`A page titled [Release notes](${pageURL}) was updated in the **Team space** space.`);
});

/**
 * @objective Verify a page moved to trash in a subscribed space posts a notification.
 */
test('notifies subscribed channel when a page is trashed', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to the space and trash a page
    const post = await notifySubscribedChannel(confluence, pageEvent('trashed'));

    // * Verify the notification says the page moved to trash
    expect(post.message).toBe(`A page titled [Release notes](${pageURL}) was moved to trash from the **Team space** space.`);
});

/**
 * @objective Verify a page restored from trash in a subscribed space posts a notification.
 */
test('notifies subscribed channel when a page is restored', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to the space and restore a page
    const post = await notifySubscribedChannel(confluence, pageEvent('restored'));

    // * Verify the notification says the page was restored
    expect(post.message).toBe(`A page titled [Release notes](${pageURL}) was restored to the **Team space** space.`);
});

/**
 * @objective Verify an event type the subscription does not include is not posted to the channel.
 */
test('skips events the subscription does not include', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to page created events only
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id, {events: ['page_created']});

    // # Update a page, then create another one
    await deliverForgeEvent(pageEvent('updated'));
    await deliverForgeEvent(pageEvent('created', 'Launch plan', longPageId));

    // * Verify only the page created notification is posted
    await waitForPostText(confluence.userClient, confluence.channel.id, 'Launch plan');
    expect((await channelPostTexts(confluence.userClient, confluence.channel.id)).filter((text) => text.includes('was updated'))).toEqual([]);
});

/**
 * @objective Verify a page subscription posts events for its page and not for other pages in the same space.
 */
test('notifies page subscription only for its page', {tag: ['@confluence_plugin', '@synthetic-inbound']}, async ({confluence}) => {
    // # Subscribe the channel to a single page
    confluence.connect();
    await subscribe(confluence.userClient, confluence.channel.id, {alias: 'release-page', page: pageId});

    // # Update another page in the space, then the subscribed page
    await deliverForgeEvent(pageEvent('updated', longPageTitle, longPageId));
    await deliverForgeEvent(pageEvent('updated'));

    // * Verify only the subscribed page's update is posted
    await waitForPostText(confluence.userClient, confluence.channel.id, 'Release notes');
    expect((await channelPostTexts(confluence.userClient, confluence.channel.id)).filter((text) => text.includes(longPageTitle))).toEqual([]);
});
