# Confluence plugin Playwright tests

Two lanes. The fake lane is what CI runs on every PR: Mattermost runs in Docker and Confluence is replaced by `fake-confluence/` (started by `global_setup.ts`), so it needs no Atlassian account or secrets. The real lane talks to our Confluence Cloud test site and runs locally or in the opt-in nightly job.

```sh
make e2e-env-up    # Postgres + Mattermost on :8065
make e2e           # builds the E2E bundle (make e2e-dist), installs the plugin, runs the suite
make e2e-env-down

make e2e-real      # normal bundle, against the test site. Needs e2e/playwright/.env
```

`make e2e-dist` points the Cloud OAuth hosts at fake-confluence and shortens the Forge poll. The plugin logs a warning on activation. Never ship that bundle.

## Environment variables

Set them in the shell or in `e2e/playwright/.env` (copy `.env.example`; docker compose reads it too). Secret values live in 1Password, `.env` (gitignored) and GitHub Actions secrets; never commit them.

### Fake lane

| Variable | Default | Purpose |
|---|---|---|
| `MM_IMAGE` | `mattermostdevelopment/mattermost-enterprise-edition:master` | Server image (runs as amd64, emulated on Apple Silicon) |
| `MM_PORT` | `8065` | Host port; must be the same for `e2e-env-up` and `e2e` |
| `PW_BASE_URL` | `http://127.0.0.1:${MM_PORT}` | Mattermost URL the tests use; leave unset unless the server runs elsewhere |
| `CONFLUENCE_FLAVOR` | `cloud` | `cloud` (OAuth 3LO + Forge drain) or `server` (Data Center OAuth + webhooks) |
| `FAKE_CONFLUENCE_PORT` | `8092` | Port fake-confluence listens on |
| `PW_HEADLESS` | `true` | `false` to watch the browser (`npm run test:headed`) |
| `PW_SLOWMO` | `0` | Delay between browser actions in ms |

`npm run test:visual` sets its own `MM_IMAGE`, `CONFLUENCE_FLAVOR`, `MM_PORT` (8078) and `PW_BROWSER_PORT` (3338); override the ports with `VISUAL_MM_PORT` and `VISUAL_BROWSER_PORT`.

### Real lane

All required unless marked otherwise. The site, regular user and OAuth app are the same ones the Jira suite uses.

| Variable | Purpose |
|---|---|
| `MM_PORT` | Must match the callback URL registered in the OAuth app: `http://127.0.0.1:${MM_PORT}/plugins/com.mattermost.confluence/api/v1/oauth2/complete.html` (CI uses 8070) |
| `CONFLUENCE_CLOUD_URL` | Test site, `https://<site>.atlassian.net` |
| `CONFLUENCE_SPACE_KEY` | Space the regular user can read; the journey subscribes a channel to it |
| `CONFLUENCE_CLOUD_USER_EMAIL` | Regular user who connects through the consent screen |
| `CONFLUENCE_CLOUD_USER_PASSWORD`, `CONFLUENCE_CLOUD_USER_TOTP_SECRET` | Regular user's Atlassian sign-in. The TOTP secret is the key behind "Can't scan the QR code?" when enrolling an authenticator app at id.atlassian.com. Optional while a fresh `.auth/` session exists |
| `CONFLUENCE_CLOUD_OAUTH_CLIENT_ID`, `CONFLUENCE_CLOUD_OAUTH_CLIENT_SECRET` | Atlassian OAuth 2.0 (3LO) app with the Confluence scopes, owned by the admin account |

In GitHub Actions these are repository secrets, except `CONFLUENCE_SPACE_KEY`, which is a variable. The nightly job also needs the variable `CONFLUENCE_REAL_E2E_ENABLED=true`.

## How Confluence is faked

- fake-confluence implements the calls the plugin makes: Cloud 3LO (`/authorize`, `/oauth/token`, accessible-resources), Data Center OAuth (`/rest/oauth2/latest/...`), the v1 REST reads, and the Forge drain.
- Mattermost resolves `fake-confluence` and `e2e.atlassian.net` via `extra_hosts`. Chromium uses `--host-resolver-rules`.
- Tests drive it through `support/fake_confluence.ts`: `reset()`, `requests()`, `enqueueForgeEvent()`, `fault()`, and `postDrain()` (HMAC).
- Inbound events (Forge drain, Data Center webhooks) are synthetic; those tests are tagged `@synthetic-inbound`.
- Only `connect.spec.ts` drives the browser OAuth flow. Other specs connect with `seedConnection()` (`support/kv.ts`). Keep it in sync with `server/store/store.go` and `server/auth_token.go`.

## Keeping the fake honest

Published specs, and where this suite does not follow them:

- **Validated:** `GET /wiki/rest/api/user/current` and `GET /wiki/rest/api/user` against [Confluence Cloud REST v1](https://developer.atlassian.com/cloud/confluence/swagger.v3.json). Server calls are the same resources without the `/wiki` prefix; Data Center does not publish an OpenAPI 3 document.
- **Allowlisted gap:** `GET /wiki/rest/api/content/{id}`, `GET /wiki/rest/api/space`, and `GET /wiki/rest/api/space/{spaceKey}` were removed from that spec. OAuth on Cloud returns 410 for them. The plugin still calls the v1 routes (classic scopes do not cover v2). Any other Confluence REST route fails the run.
- **Not in a Confluence OpenAPI spec:** Cloud 3LO and Data Center `/rest/oauth2` (Atlassian identity / the OAuth plugin), and the Forge drain. Forge events follow the [Forge Confluence event reference](https://developer.atlassian.com/platform/forge/events-reference/confluence/). The drain checks `X-MM-Signature` (HMAC-SHA256 of the body). Server webhooks are the shared-secret query parameter from Confluence's webhook screen, not an HMAC.
- New route: add it to `fake-confluence/contract/operations.json`, run `npm run contract:update`, commit `fake-confluence/contract/specs/`.
- Nightly, `npm run contract:check` fails if Atlassian changed the committed operations upstream.

## Real Confluence

The regular user signs in on Atlassian's own screens with a password and a two-step verification code generated from `CONFLUENCE_CLOUD_USER_TOTP_SECRET` (`real-confluence/login/`). The session is saved to `.auth/` and refreshed once it is 3 days old; `npm run real:login` forces a fresh one. Traces, screenshots, and video are off when `CI` is set. Do not put account names in this tree.

## Visual tests

`tests/visual.spec.ts` (`@visual`) screenshots the subscription modal and page notifications and compares them with the PNGs in `tests/__screenshots__/`. `npm run test:visual` starts a separate stack with a pinned server image plus a dockerized browser, so baselines match on any machine and in CI (a non-blocking job). After an intended UI change, run `npm run test:visual-update-snapshots`, look at the new PNGs, and commit them. Bump the pinned image only together with a baseline update.

## Writing tests

Tests follow the [Mattermost Playwright guidelines](https://github.com/mattermost/mattermost/blob/master/e2e-tests/playwright/CLAUDE.md): a JSDoc `@objective` (and `@precondition` when setup is unusual), an action-oriented title, the `@confluence_plugin` tag, and `// #` action / `// *` verification comments. Prefer page objects and `getByRole`/`getByText` locators over CSS.

## Gotchas

- Don't use `pw.initSetup()`: it resets server config and disables the plugin. Use the `confluence` fixture instead.
