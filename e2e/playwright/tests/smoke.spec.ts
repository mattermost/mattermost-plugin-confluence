// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {pluginId} from '../support/constants';
import {expect, test} from '../support/fixture';
import {openChannel} from '../support/ui';

/**
 * @objective Verify the plugin's server side is running after install.
 */
test('reports plugin server as running', {tag: '@confluence_plugin'}, async ({confluence}) => {
    // # Fetch the plugin statuses
    const statuses = await confluence.adminClient.getPluginStatuses();
    const status = statuses.find((s) => s.plugin_id === pluginId);

    // * Verify the plugin is running
    expect(status?.state).toBe(2);
});

/**
 * @objective Verify the plugin's webapp bundle registers without page or console errors.
 */
test('loads webapp bundle without errors', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Collect page errors and console errors raised from the plugin's bundle while a channel loads
    const {page} = await pw.testBrowser.login(confluence.user);
    const errors: string[] = [];
    page.on('pageerror', (err) => errors.push(`pageerror: ${err.message}`));
    page.on('console', (msg) => {
        if (msg.type() === 'error' && (msg.location().url.includes(`/plugins/${pluginId}/`) || msg.text().toLowerCase().includes('confluence'))) {
            errors.push(`console: ${msg.text()} (${msg.location().url})`);
        }
    });
    await page.goto(`/${confluence.team.name}/channels/town-square`);
    await page.waitForFunction((id) => Boolean((window as any).plugins?.[id]), pluginId);

    // * Verify nothing was logged
    expect(errors).toEqual([]);
});

/**
 * @objective Verify the /confluence slash command offers its subcommands in autocomplete.
 */
test('autocompletes /confluence subcommands in the message box', {tag: '@confluence_plugin'}, async ({pw, confluence}) => {
    // # Type /confluence in the message box
    const {channelsPage} = await openChannel(pw, confluence.user, confluence.team.name);
    const postCreate = channelsPage.centerView.postCreate;
    await postCreate.input.fill('/confluence ');

    // * Verify connect is suggested
    await expect(postCreate.suggestionList).toContainText('connect');
});
