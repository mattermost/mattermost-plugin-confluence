// Structural validation for manifest.yml. Subset of `forge lint` that runs
// without Atlassian credentials so CI can catch broken function references,
// missing events, and empty egress allowlists.

import * as fs from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';

type Manifest = {
    modules?: {
        trigger?: { key: string; function: string; events: string[] }[];
        webtrigger?: { key: string; function: string }[];
        function?: { key: string; handler: string }[];
    };
    app?: { id?: string };
    permissions?: {
        external?: { fetch?: { backend?: string[] } };
    };
};

const PLACEHOLDER_APP_ID = 'REPLACE_WITH_FORGE_APP_ID';

const manifestPath = path.resolve(__dirname, '..', 'manifest.yml');
const raw = fs.readFileSync(manifestPath, 'utf8');
const manifest = yaml.load(raw) as Manifest;

const errors: string[] = [];
const warnings: string[] = [];

const triggers = manifest.modules?.trigger ?? [];
const webtriggers = manifest.modules?.webtrigger ?? [];
const functions = manifest.modules?.function ?? [];

const declaredFns = new Set(functions.map((f) => f.key));
const referencedFns = new Set<string>();

for (const t of triggers) {
    referencedFns.add(t.function);
    if (!declaredFns.has(t.function)) {
        errors.push(`trigger "${t.key}" references undefined function "${t.function}"`);
    }
    if (!t.events || t.events.length === 0) {
        errors.push(`trigger "${t.key}" has no events`);
    }
}

for (const w of webtriggers) {
    referencedFns.add(w.function);
    if (!declaredFns.has(w.function)) {
        errors.push(`webtrigger "${w.key}" references undefined function "${w.function}"`);
    }
}

for (const f of functions) {
    if (!referencedFns.has(f.key)) {
        warnings.push(`function "${f.key}" is declared but not referenced by any trigger/webtrigger`);
    }
}

// Pull architecture: the Forge app must NOT declare external egress. The
// Mattermost plugin polls a drain web trigger instead. Any fetch.backend entry
// would force per-customer manifest editing and break the "one deploy, many
// installs" distribution model.
const fetchBackend = manifest.permissions?.external?.fetch?.backend ?? [];
if (fetchBackend.length > 0) {
    errors.push(
        'permissions.external.fetch.backend must be empty (pull architecture, no egress); ' +
            `found: ${JSON.stringify(fetchBackend)}`,
    );
}

if (manifest.app?.id?.includes(PLACEHOLDER_APP_ID)) {
    warnings.push(`app.id still has the placeholder "${PLACEHOLDER_APP_ID}" — expected before first deploy`);
}

if (warnings.length > 0) {
    console.warn('warnings:');
    for (const w of warnings) {
        console.warn(`  - ${w}`);
    }
}

if (errors.length > 0) {
    console.error('errors:');
    for (const e of errors) {
        console.error(`  - ${e}`);
    }
    process.exit(1);
}

console.log(
    `manifest ok: ${triggers.length} triggers, ${webtriggers.length} webtriggers, ${functions.length} functions`,
);
