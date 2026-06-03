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

Customers with strict data-residency or compliance requirements (most
often on-prem Mattermost deployments) can run the bridge under their
own Atlassian developer account instead of using the Mattermost-published
one. The full bridge source is in this directory. After you deploy it
yourself, point the plugin at your install link via
**System Console → Plugins → Confluence → Forge Bridge Install URL**.

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

## Customer workflow (each Confluence Cloud tenant — one-time)

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
