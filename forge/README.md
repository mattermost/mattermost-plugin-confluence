# Mattermost Confluence Forge bridge

This Forge app is the GA replacement for the Atlassian Connect webhook
descriptor that Atlassian closed for new installs on March 31, 2026.
It is required for new Confluence **Cloud** customers to receive
events in Mattermost. Confluence **Server / Data Center** installs are
unaffected and continue to use the existing webhook path.

## Who runs the bridge

Each customer runs their own bridge under their own Atlassian developer
account. Mattermost does not publish a shared bridge for customer use:
Atlassian's free Forge usage limits would cap the number of tenants we
could realistically support from a single Mattermost-owned bridge, and
self-hosting also gives customers full control over the bridge's
storage and the install consent screen presented to their Confluence
admins.

The plugin is agnostic to who owns the Forge app — the setup wizard
accepts any valid Forge web trigger URL.

## Self-host runbook

### Prerequisites

- An [Atlassian developer account](https://developer.atlassian.com/console/myapps/)
  for your organisation. Free to create; one account can host the
  bridge app for all your Confluence Cloud tenants.
- Node.js 22 (LTS) or newer.
- The Forge CLI: `npm install -g @forge/cli`. Run `forge login` once
  with an API token from
  [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).
- Confluence Cloud Site Admin on the target tenant.
- Mattermost System Admin on the target server.

### Step 1 — Get the bridge source

Clone the plugin repository and change into the `forge/` directory:

```bash
git clone https://github.com/mattermost/mattermost-plugin-confluence.git
cd mattermost-plugin-confluence/forge
npm install
```

### Step 2 — Register the app under your developer account

```bash
forge register
```

The CLI will prompt for an app name (e.g. `acme-mattermost-confluence-bridge`)
and write the generated app ID into `manifest.yml` under `app.id`. This
ID is yours; do not share the file or the ID outside your organisation.

### Step 3 — Deploy

```bash
forge deploy --environment production
```

This publishes the app to your developer account. It is not visible
to anyone outside your organisation and is not listed on the Atlassian
Marketplace.

### Step 4 — Generate a private install link

In the
[Atlassian developer console](https://developer.atlassian.com/console/myapps/),
open the app you just deployed → **Distribution** → **Sharing** →
generate a private install link. This link is what your Confluence
Site Admins (or you, if you are the only tenant) will click to install
the bridge.

### Step 5 — Tell the Mattermost plugin where the install link lives

In Mattermost: **System Console → Plugins → Confluence → Forge Bridge
Install URL**. Paste the install link from the previous step. Save.

You only need to do this once per Mattermost server; the wizard will
surface this URL to the admin running the Cloud setup.

### Step 6 — Install the bridge on your Confluence Cloud site

The Confluence Site Admin clicks the install link from step 4, reviews
the consent screen (read access to pages and comments, no outbound
network), and approves the install. Atlassian provisions a per-tenant
copy of the app: isolated storage, unique web trigger URLs, no shared
state with any other tenant.

### Step 7 — Get the bridge's web trigger URLs

From a terminal authenticated with `forge login` and the same developer
account that owns the app:

```bash
forge webtrigger --environment production
```

Pick the installed tenant when prompted. The CLI prints two URLs:
- `drain` → the URL Mattermost will poll
- `register` → a one-shot URL used to set the shared secret

### Step 8 — Run the Mattermost setup wizard

In Mattermost, run `/confluence install cloud`. Step through the
wizard; when it reaches the **Forge bridge** step, click **Register
bridge** and paste:
- `drain` URL from step 7 → "Drain URL"
- `register` URL from step 7 → "Register URL"

The plugin POSTs its auto-generated shared secret to your bridge's
register endpoint, stores the drain URL in plugin config, and starts
polling on a 30-second tick.

### Verify

In Confluence, edit a page in a space subscribed in Mattermost. Within
~30 seconds, the subscribed channel should receive the page-edit
notification.

### Operational notes

- `register` is one-shot. If you need to rotate the shared secret,
  clear `mm.registered` from Forge storage first (use `forge install
  --upgrade` after manually wiping the entry), then re-run the
  Mattermost wizard.
- Forge storage values are capped at 240 KiB per entry. The bridge
  drops the inline page body for events that would exceed this; the
  channel notification still fires but @-mention DMs are skipped for
  that single oversized event.
- Forge web trigger throttle is 1000 req/min per app/environment. At
  a 30-second poll cadence that is 2 req/min per tenant, so one bridge
  accommodates ~500 Confluence Cloud tenants before throttling.
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

## Mattermost-internal QA bridge

A Mattermost-owned copy of this app lives in our Atlassian developer
space for internal QA and demos only. It is **not** distributed to
customers, not surfaced in the marketplace, and not the path documented
to end users. Customers always self-host (see runbook above).

Internal deploy:

1. `npm install` in this directory.
2. `forge login` against the shared Mattermost Atlassian developer
   account, then `forge deploy --environment production`.
3. `forge install --site https://<internal-test-tenant>.atlassian.net`.
4. Smoke-test with a Mattermost dev server pointed at the resulting
   `drain` / `register` web trigger URLs.

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
