# Mattermost Confluence Forge bridge

This Forge app is the GA replacement for the Atlassian Connect webhook
descriptor that Atlassian closed for new installs on March 31, 2026.

## What this means for your Confluence site

Mattermost publishes one Forge app, and every customer Confluence Cloud
site installs its own private copy of it. Atlassian enforces strict
isolation between those copies, so:

- Your queued Confluence events, the secret that protects your bridge,
  and any data the bridge holds live only inside your tenant's copy.
  Other Mattermost customers cannot see them, and Mattermost staff
  cannot read them.
- The bridge does not store Confluence page content anywhere. It only
  forwards events into your own Mattermost server when your Mattermost
  server asks for them.
- Each tenant gets its own bridge URLs. There is no single URL that
  could be used to read another customer's events.
- The shared secret used to authenticate Mattermost to the bridge is
  set once by your admin during install and is never sent to or stored
  by Mattermost-the-company.

### If you would rather run the bridge yourself

Customers with strict data-residency / compliance requirements, or
on-prem Mattermost deployments at a scale that would exceed the free
Forge usage limits on the Mattermost-published bridge, can run the
bridge themselves under their own Atlassian developer account. The
Mattermost plugin treats both paths identically — the wizard accepts
any valid Forge web trigger URL regardless of which Atlassian account
owns the app.

#### Prerequisites

- An [Atlassian developer account](https://developer.atlassian.com/console/myapps/)
  for your organisation. Free to create; one account can host the
  bridge app for all your Confluence Cloud tenants.
- Node.js 22 (LTS) or newer.
- The Forge CLI: `npm install -g @forge/cli`. Run `forge login` once
  with an API token from
  [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
- Confluence Cloud Site Admin on the target tenant.
- Mattermost System Admin on the target server.

#### Step 1 — Get the bridge source

Clone the plugin repository and change into the `forge/` directory:

```bash
git clone https://github.com/mattermost/mattermost-plugin-confluence.git
cd mattermost-plugin-confluence/forge
npm install
```

#### Step 2 — Register the app under your developer account

```bash
forge register
```

The CLI will prompt for an app name (e.g. `acme-mattermost-confluence-bridge`)
and write the generated app ID into `manifest.yml` under `app.id`. This
ID is yours; do not share the file or the ID outside your organisation.

#### Step 3 — Deploy

```bash
forge deploy --environment production
```

This publishes the app to your developer account. It is not visible
to anyone outside your organisation and is not listed on the Atlassian
Marketplace.

#### Step 4 — Generate a private install link

In the
[Atlassian developer console](https://developer.atlassian.com/console/myapps/),
open the app you just deployed → **Distribution** → **Sharing** →
generate a private install link. This link is what your Confluence
Site Admins (or you, if you are the only tenant) will click to install
the bridge.

#### Step 5 — Tell the Mattermost plugin where the install link lives

In Mattermost: **System Console → Plugins → Confluence → Forge Bridge
Install URL**. Paste the install link from the previous step. Save.

You only need to do this once per Mattermost server; the wizard will
surface this URL to the admin running the Cloud setup.

#### Step 6 — Install the bridge on your Confluence Cloud site

The Confluence Site Admin clicks the install link from step 4, reviews
the consent screen (read access to pages and comments, no outbound
network), and approves the install. Atlassian provisions a per-tenant
copy of the app: isolated storage, unique web trigger URLs, no shared
state with any other tenant.

#### Step 7 — Get the bridge's web trigger URLs

From a terminal authenticated with `forge login` and the same developer
account that owns the app:

```bash
forge webtrigger --environment production
```

Pick the installed tenant when prompted. The CLI prints two URLs:
- `drain` → the URL Mattermost will poll
- `register` → a one-shot URL used to set the shared secret

#### Step 8 — Run the Mattermost setup wizard

In Mattermost, run `/confluence install cloud`. Step through the
wizard; when it reaches the **Forge bridge** step, click **Register
bridge** and paste:
- `drain` URL from step 7 → "Drain URL"
- `register` URL from step 7 → "Register URL"

The plugin POSTs its auto-generated shared secret to your bridge's
register endpoint, stores the drain URL in plugin config, and starts
polling on a 30-second tick.

#### Verify

In Confluence, edit a page in a space subscribed in Mattermost. Within
~30 seconds, the subscribed channel should receive the page-edit
notification.

#### Operational notes

- `register` is one-shot. If you need to rotate the shared secret,
  clear `mm.registered` from Forge storage first (use `forge install
  --upgrade` after manually wiping the entry), then re-run the
  Mattermost wizard.
- Forge storage values are capped at 240 KiB per entry. The bridge
  drops the inline page body for events that would exceed this; the
  channel notification still fires but @-mention DMs are skipped for
  that single oversized event.
- Forge web trigger throttle is 1000 req/min per app/environment. At
  a 30-second poll cadence that is 2 req/min per tenant, so a single
  self-hosted bridge accommodates ~500 Confluence Cloud tenants before
  throttling.
- Forge storage is wiped 28 days after the app is uninstalled. The
  bridge is a buffer, not a system of record; the Mattermost plugin
  is the durable side.

## Shape

This is a **pull** bridge:

- The Forge app subscribes to 8 Confluence events via `trigger` modules
  (page `created`/`updated`/`trashed`/`restored`/`deleted`, comment
  `created`/`updated`/`deleted`) and enqueues each event payload into Forge
  storage under `evt:<cloudId>:<ts>:<rand>`.
- The Mattermost plugin periodically POSTs to the `drain` web trigger to
  read queued events and ack them. Requests are HMAC-SHA256 signed using a
  shared secret the admin sets via the one-shot `register` web trigger.
- No `permissions.external.fetch` is declared. The Forge app never makes
  outbound calls. This keeps the install consent screen clean and removes
  the per-customer `manifest.yml` editing the previous push design required.

Trade-off: Atlassian's Forge `trigger` module already has up to 3 minutes
of delivery delay, so the additional ~30s polling latency we add on the
plugin side is small in context.

## Operator workflow (Mattermost — one-time, ships the app)

1. `npm install` in this directory.
2. `forge login` and `forge register` (one-time, generates the app ID —
   paste it into `manifest.yml` under `app.id`).
3. `forge deploy --environment production`.
4. `forge install --site https://<test-tenant>.atlassian.net` to verify
   on a test tenant.
5. From the Atlassian developer console, generate a private distribution
   link and publish it as the `Forge Bridge Install URL` plugin setting
   default for downstream Mattermost installs.

## Customer workflow — Mattermost-published bridge

This is the short path for tenants installing the bridge published by
Mattermost (typically used for trials, evaluation, and small deployments;
self-hosting is recommended for production scale — see the "If you
would rather run the bridge yourself" section above for the full
runbook).

1. Confluence admin clicks the install link from the Mattermost setup
   wizard → app installs on their site (Atlassian creates a per-tenant
   install with isolated storage and unique web trigger URLs).
2. Confluence admin runs `forge webtrigger` (or reads the install logs)
   to get the `register` and `drain` URLs. We'll wrap this in a UI Kit
   admin page in a follow-up.
3. In Mattermost System Console under Plugins > Confluence:
   - Paste the `drain` URL into "Forge Drain URL".
   - Copy the auto-generated "Forge Bridge Shared Secret".
4. POST the secret to the `register` URL once:

   ```bash
   curl -X POST -H 'Content-Type: application/json' \
     -d '{"secret":"<paste shared secret>"}' \
     '<register-web-trigger-url>'
   ```

   `register` is one-shot — it refuses subsequent calls so a leaked URL
   can't be used to repoint the bridge. To re-register (e.g. rotate the
   secret), clear `mm.registered` from Forge storage first.

The Mattermost plugin then polls `drain` on a ticker, verifies each
event, posts to subscribed channels, and acks drained keys so Forge can
delete them.

## Develop

This directory is its own Node project, independent of the plugin's
`server/` and `webapp/` builds. CI lives in `.github/workflows/forge-ci.yml`
and only fires when `forge/**` changes.

```bash
npm install --omit=optional   # CI install path
npm install                   # developer install path, pulls @forge/cli

npm run typecheck
npm run validate-manifest
npm run build
npm run ci                    # all of the above
npm run deploy                # forge deploy
```

## Why these 8 events

Direct one-to-one mapping with what the legacy Atlassian Connect descriptor
used to subscribe to (page + comment lifecycle), re-validated against
[the Forge Confluence events list](https://developer.atlassian.com/platform/forge/events-reference/confluence/).
Forge collapses Connect's `removed` onto `deleted`. See
`server/forge_event_mapping.go` on the plugin side for the explicit mapping.

## Known limits

- Forge `trigger` delivery is best-effort (up to ~3 min delay, occasional
  drops). Connect webhooks had the same property, so we are not
  regressing. If drops show up in production we will add a plugin-side
  reconciliation poll over `/wiki/api/v2/pages?sort=-modified-date`.
- Forge storage is wiped 28 days after uninstall — the queue is buffer,
  not durable state. The plugin is the system of record.
- Forge web trigger limit: 1000 req/min per app/env/context. At a 30s
  poll cadence, that's 2 req/min per tenant → headroom for ~500
  installations per environment before throttling.

## Not in scope here

- Plugin-side Cloud 3LO OAuth (lives in `server/instance_cloud.go` and
  the Cloud branches of `server/user.go` / `server/flow.go`).
- Plugin-side polling loop (lives in `server/forge_poller.go`).
- Migration of existing Connect installs (we leave those running until
  Atlassian's Q4 2026 EOS).
