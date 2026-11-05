// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export type ConfluenceUser = {
    type: 'known';
    username: string;
    userKey: string;
    accountId: string;
    accountType: 'atlassian';
    displayName: string;
    publicName: string;
};

export type JournalEntry = {
    method: string;
    path: string;
    query: Record<string, string>;
    body: unknown;
};

export type ForgeEvent = {
    eventType: string;
    atlassianId: string;
    context: {cloudId: string; moduleKey: string};
    content: {
        id: string;
        type: string;
        title: string;
        status: string;
        space: {key: string; name: string};
        container?: {id: string; type: string; title: string; space: {key: string; name: string}};
        extensions?: {location: string};
    };
};

export type PageAction = 'created' | 'updated' | 'trashed' | 'restored';

export type PageRecord = {
    id: string;
    title: string;
    body: string;
    spaceKey: string;
};

type SpaceRecord = {id: number; key: string; name: string};

export type FakeConfluenceState = {
    user: ConfluenceUser;
    spaces: SpaceRecord[];
    pages: Record<string, PageRecord>;
    codes: Map<string, string>;
    journal: JournalEntry[];
    contractViolations: string[];
    faults: Map<string, number>;
    forgeEvents: Array<{key: string; event: ForgeEvent}>;
    nextForgeKey: number;
};

export const cloudId = 'e2e-cloud-id';

export const defaultUser = (): ConfluenceUser => ({
    type: 'known',
    username: 'e2euser',
    userKey: 'e2e-user-key',
    accountId: 'e2e-account-id',
    accountType: 'atlassian',
    displayName: 'E2E User',
    publicName: 'E2E User',
});

export const teamSpace: SpaceRecord = {id: 2001, key: 'TEAM', name: 'Team space'};

export const longPageTitle = 'Quarterly cross-team planning sync for the platform reliability roadmap '.repeat(4).trim();

export function defaultPage(id: string, title: string): PageRecord {
    return {id, title, body: '<p>Ship notes for the release.</p>', spaceKey: teamSpace.key};
}

export function defaultState(): FakeConfluenceState {
    return {
        user: defaultUser(),
        spaces: [teamSpace],
        pages: {
            '1001': defaultPage('1001', 'Release notes'),
            '1002': defaultPage('1002', longPageTitle),
        },
        codes: new Map(),
        journal: [],
        contractViolations: [],
        faults: new Map(),
        forgeEvents: [],
        nextForgeKey: 1,
    };
}

export function pageEvent(action: PageAction, title = 'Release notes', id = '1001'): ForgeEvent {
    return {
        eventType: `avi:confluence:${action}:page`,
        atlassianId: defaultUser().accountId,
        context: {cloudId, moduleKey: 'e2e'},
        content: {
            id,
            type: 'page',
            title,
            status: action === 'trashed' ? 'trashed' : 'current',
            space: {key: teamSpace.key, name: teamSpace.name},
        },
    };
}

export function pageCreatedEvent(title = 'Release notes', id = '1001'): ForgeEvent {
    return pageEvent('created', title, id);
}

export function commentCreatedEvent(pageTitle = 'Release notes', pageID = '1001', commentID = '3001'): ForgeEvent {
    const space = {key: teamSpace.key, name: teamSpace.name};
    return {
        eventType: 'avi:confluence:created:comment',
        atlassianId: defaultUser().accountId,
        context: {cloudId, moduleKey: 'e2e'},
        content: {
            id: commentID,
            type: 'comment',
            title: `Re: ${pageTitle}`,
            status: 'current',
            space,
            container: {id: pageID, type: 'page', title: pageTitle, space},
            extensions: {location: 'footer'},
        },
    };
}
