// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Client4} from '@mattermost/client';
import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';
import {expect, type ChannelsPage, type PlaywrightExtended} from '@mattermost/playwright-lib';
import type {Page} from '@playwright/test';

export async function openChannel(pw: PlaywrightExtended, user: UserProfile, teamName: string, channelName = 'town-square') {
    const {channelsPage, page} = await pw.testBrowser.login(user);
    await channelsPage.goto(teamName, channelName);
    await channelsPage.toBeVisible();
    return {channelsPage, page};
}

export async function runSlashCommand(channelsPage: ChannelsPage, command: string) {
    const postCreate = channelsPage.centerView.postCreate;
    await postCreate.input.fill(command);

    // Enter would pick the highlighted slash-command autocomplete suggestion instead of sending.
    await postCreate.sendMessageButton.click();
    await expect(postCreate.input).toHaveValue('');
}

export function postText(post: Post) {
    const attachments = (post.props?.attachments as Array<{pretext?: string; title?: string; text?: string}> | undefined) ?? [];
    return [post.message, ...attachments.flatMap((attachment) => [attachment.pretext, attachment.title, attachment.text])].filter(Boolean).join('\n');
}

export async function waitForPostText(client: Client4, channelId: string, text: string) {
    let found: Post | undefined;
    await expect.poll(async () => {
        const {order, posts} = await client.getPosts(channelId);
        found = order.map((id) => posts[id]).find((post) => postText(post).includes(text));
        return Boolean(found);
    }, {timeout: 20_000}).toBe(true);
    return found!;
}
