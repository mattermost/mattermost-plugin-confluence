// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'node:fs';
import path from 'node:path';

import Ajv, {ValidateFunction} from 'ajv';
import addFormats from 'ajv-formats';

type Operation = {
    specName: string;
    method: string;
    template: string;
    pattern: RegExp;
    requestBody?: {content?: Record<string, {schema?: object}>};
    responses: Record<string, {content?: Record<string, {schema?: object}>}>;
};

const specsDir = path.join(__dirname, 'specs');

// v1 content and space reads were removed from the published Cloud spec (deprecated, and 410 for OAuth).
// The Data Center client still calls them; on Cloud the fake answers 410. Anything else not in operations.json fails the suite.
const allowlistedGaps = [
    {method: 'GET', pattern: /^\/wiki\/rest\/api\/content\/[^/]+$/},
    {method: 'GET', pattern: /^\/wiki\/rest\/api\/space\/?$/},
    {method: 'GET', pattern: /^\/wiki\/rest\/api\/space\/[^/]+$/},
];

export class ContractValidator {
    private readonly ajv = new Ajv({strict: false, allErrors: true, verbose: true});
    private readonly operations: Operation[] = [];
    private readonly compiled = new Map<string, ValidateFunction>();

    constructor() {
        addFormats(this.ajv);
        for (const format of ['int32', 'int64', 'float', 'double', 'binary']) {
            this.ajv.addFormat(format, true);
        }

        for (const file of fs.readdirSync(specsDir).filter((f) => f.endsWith('.json'))) {
            const specName = path.basename(file, '.json');
            const spec = JSON.parse(fs.readFileSync(path.join(specsDir, file), 'utf8'));
            this.ajv.addSchema({components: spec.components}, specName);

            for (const [template, methods] of Object.entries<Record<string, Omit<Operation, 'specName' | 'method' | 'template' | 'pattern'>>>(spec.paths)) {
                for (const [method, op] of Object.entries(methods)) {
                    this.operations.push({specName, method: method.toUpperCase(), template, pattern: templateToRegExp(template), ...op});
                }
            }
        }
    }

    // spec: published operation. allowlisted: known gap. missing: a route the fake should not be serving unmarked.
    coverage(method: string, requestPath: string): 'spec' | 'allowlisted' | 'missing' {
        if (this.find(method, requestPath)) {
            return 'spec';
        }
        if (allowlistedGaps.some((gap) => gap.method === method && gap.pattern.test(requestPath))) {
            return 'allowlisted';
        }
        return 'missing';
    }

    validateRequest(method: string, requestPath: string, body: unknown): string[] {
        const op = this.find(method, requestPath);
        if (!op) {
            if (this.coverage(method, requestPath) === 'allowlisted') {
                return [];
            }
            return [`${method} ${requestPath} is not covered by fake-confluence/contract/operations.json`];
        }
        const schema = op.requestBody?.content?.['application/json']?.schema;
        if (!schema || body === undefined) {
            return [];
        }
        return this.validate(op, 'request', schema, body);
    }

    validateResponse(method: string, requestPath: string, status: number, body: unknown): string[] {
        const op = this.find(method, requestPath);
        if (!op) {
            return [];
        }
        const response = op.responses[String(status)] ?? op.responses.default;
        if (!response) {
            return [`${op.method} ${op.template}: status ${status} is not a documented response`];
        }
        const schema = response.content?.['application/json']?.schema;
        if (!schema) {
            return [];
        }
        return this.validate(op, `response ${status}`, schema, body);
    }

    private find(method: string, requestPath: string) {
        return this.operations.find((op) => op.method === method && op.pattern.test(requestPath));
    }

    private validate(op: Operation, label: string, schema: object, body: unknown): string[] {
        const key = `${op.specName} ${op.method} ${op.template} ${label}`;
        let validate = this.compiled.get(key);
        if (!validate) {
            validate = this.ajv.compile(rewriteRefs(schema, op.specName));
            this.compiled.set(key, validate);
        }
        if (validate(body)) {
            return [];
        }
        return (validate.errors ?? []).map((e) => `${op.method} ${op.template} ${label}: ${e.instancePath || '/'} ${e.message} ${JSON.stringify(e.params)}`);
    }
}

function templateToRegExp(template: string) {
    const escaped = template.replace(/[.*+?^$()|[\]\\]/g, '\\$&').replace(/\{[^}]+\}/g, '[^/]+');
    return new RegExp(`^${escaped}$`);
}

function rewriteRefs(schema: object, specName: string): object {
    return JSON.parse(JSON.stringify(schema).replaceAll('"#/components/', `"${specName}#/components/`));
}
