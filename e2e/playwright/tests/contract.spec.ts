// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@playwright/test';

import {ContractValidator} from '../fake-confluence/contract/validator';
import {defaultUser} from '../fake-confluence/state';

const validator = new ContractValidator();

/**
 * @objective Verify the fake's default user matches Atlassian's published user schema.
 */
test('validates fake user against the published user schema', {tag: '@confluence_plugin'}, () => {
    // # Validate the fake's current-user response
    const violations = validator.validateResponse('GET', '/wiki/rest/api/user/current', 200, defaultUser());

    // * Verify there are no violations
    expect(violations).toEqual([]);
});

/**
 * @objective Verify the validator catches a response that breaks the published schema, so a passing check means something.
 */
test('rejects user response missing the required type', {tag: '@confluence_plugin'}, () => {
    // # Drop the required type property from the user
    const {type, ...withoutType} = defaultUser();
    const violations = validator.validateResponse('GET', '/wiki/rest/api/user/current', 200, withoutType);

    // * Verify the missing property is reported
    expect(violations.join('\n')).toContain("must have required property 'type'");
    expect(type).toBe('known');
});

/**
 * @objective Verify the v1 content and space reads removed from the spec stay allowlisted, while unknown routes fail.
 */
test('allowlists removed v1 reads and rejects unknown routes', {tag: '@confluence_plugin'}, () => {
    // * Verify the removed v1 reads are allowlisted and the user read is validated against the spec
    expect(validator.coverage('GET', '/wiki/rest/api/content/1001')).toBe('allowlisted');
    expect(validator.coverage('GET', '/wiki/rest/api/space')).toBe('allowlisted');
    expect(validator.coverage('GET', '/wiki/rest/api/space/TEAM')).toBe('allowlisted');
    expect(validator.coverage('GET', '/wiki/rest/api/user/current')).toBe('spec');

    // * Verify a route outside operations.json is reported
    expect(validator.validateRequest('GET', '/wiki/rest/api/not-a-real-endpoint', undefined)).toEqual([
        'GET /wiki/rest/api/not-a-real-endpoint is not covered by fake-confluence/contract/operations.json',
    ]);
});
