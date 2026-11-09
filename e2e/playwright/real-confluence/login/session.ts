// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/* eslint-disable no-console */

import fs from 'node:fs';
import path from 'node:path';

import {chromium, expect, type BrowserContext, type Locator, type Page} from '@playwright/test';

import {realConfluence} from '../env';

import {generateTotp} from './totp';

const sessionMaxAgeMs = 3 * 24 * 60 * 60 * 1000;

function credential(name: 'password' | 'totpSecret') {
    const value = realConfluence.user[name];
    if (!value) {
        throw new Error('Atlassian asked the regular test user to sign in, but CONFLUENCE_CLOUD_USER_PASSWORD and CONFLUENCE_CLOUD_USER_TOTP_SECRET ' +
            '(the two-step verification setup key) are not both set.');
    }
    return value;
}

export async function signInToAtlassian(page: Page, timeoutMs: number) {
    const siteHost = new URL(realConfluence.siteURL).hostname;
    await page.goto(`https://id.atlassian.com/login?continue=${encodeURIComponent(`${realConfluence.siteURL}/wiki`)}`);
    await completeSignIn(page, timeoutMs, async () => {
        const url = new URL(page.url());
        return url.hostname === siteHost && !url.pathname.includes('login');
    });
}

// Answers whichever Atlassian sign-in screen is showing until isDone() holds. Later steps come first because
// earlier inputs can stay on the page after they're submitted.
export async function completeSignIn(page: Page, timeoutMs: number, isDone: () => Promise<boolean>) {
    let lastCode = '';

    const steps: Array<[Locator, (el: Locator) => Promise<void>]> = [
        [page.locator('input[name="otpCode"], input[autocomplete="one-time-code"]').first(), async (el) => {
            // Atlassian rejects a code that was already used, so wait for the next 30s window.
            let code = generateTotp(credential('totpSecret'));
            while (code === lastCode) {
                await page.waitForTimeout(2000);
                code = generateTotp(credential('totpSecret'));
            }
            lastCode = code;
            await el.fill(code);
            await page.keyboard.press('Enter');
        }],
        [page.locator('input[name="password"]').first(), async (el) => {
            await el.fill(credential('password'));
            await page.keyboard.press('Enter');
        }],
        [page.locator('input[name="username"]').first(), async (el) => {
            await el.fill(realConfluence.user.email);
            await page.keyboard.press('Enter');
        }],
    ];

    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        if (await isDone()) {
            return;
        }
        const url = new URL(page.url());
        if (url.hostname === 'id.atlassian.com' && url.pathname === '/error') {
            throw new Error(`Atlassian refused the request: ${url.searchParams.get('error')} ${url.searchParams.get('error_description') ?? ''}`);
        }
        for (const [el, act] of steps) {
            if (await el.isVisible().catch(() => false) && await el.isEditable().catch(() => false)) {
                console.log(`Sign-in step: ${el}`);
                await act(el);
                await page.waitForLoadState('domcontentloaded').catch(() => undefined);
                break;
            }
        }
        await page.waitForTimeout(1000);
    }
    throw new Error(`Sign-in did not finish; last page was ${page.url()}`);
}

export async function saveSession(page: Page) {
    fs.mkdirSync(path.dirname(realConfluence.userSessionPath), {recursive: true});
    await page.context().storageState({path: realConfluence.userSessionPath});
}

export async function ensureAtlassianSession() {
    const age = fs.existsSync(realConfluence.userSessionPath) ? Date.now() - fs.statSync(realConfluence.userSessionPath).mtimeMs : Infinity;
    if (age < sessionMaxAgeMs) {
        return;
    }
    credential('password');
    credential('totpSecret');

    const browser = await chromium.launch();
    try {
        const page = await browser.newPage();
        await signInToAtlassian(page, 2 * 60_000);
        await saveSession(page);
        console.log('Signed in to Atlassian and saved the session');
    } finally {
        await browser.close();
    }
}

export async function addAtlassianSession(context: BrowserContext) {
    if (!fs.existsSync(realConfluence.userSessionPath)) {
        throw new Error('No saved Atlassian session; run `npm run real:login` and sign in as the regular test user');
    }
    const {cookies} = JSON.parse(fs.readFileSync(realConfluence.userSessionPath, 'utf8'));
    await context.addCookies(cookies);
}

// Atlassian's 3LO screen: sign in if Atlassian asks, pick the site when asked, then Accept.
export async function approveAtlassianConsent(page: Page) {
    const accept = page.getByRole('button', {name: /^accept$/i});
    await completeSignIn(page, 2 * 60_000, () => accept.isVisible());
    await expect(accept, 'Atlassian consent screen').toBeVisible({timeout: 30_000});

    const siteHost = new URL(realConfluence.siteURL).hostname;
    const sitePicker = page.getByRole('combobox');
    if (await sitePicker.count() && !(await sitePicker.first().textContent())?.includes(siteHost)) {
        await sitePicker.first().click();
        await page.getByRole('option', {name: new RegExp(siteHost.split('.')[0], 'i')}).click();
    }
    await accept.click();
}
