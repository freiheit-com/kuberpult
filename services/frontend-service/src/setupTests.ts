/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import '@testing-library/jest-dom';
import 'react-use-sub/test-util';
import { Lock, Release } from './api/api';
import { DisplayLock, UpdateFrontendConfig, UpdateOverview, updateTag } from './ui/utils/store';

// test utility to await all running promises
global.nextTick = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));

// CSS.supports is required a dependency of react-tooltip
// @ts-ignore
global.CSS = { supports: jest.fn() };

export const documentQuerySelectorSafe = (selectors: string): HTMLElement => {
    const result = document.querySelector(selectors);
    if (!result) {
        throw new Error('documentQuerySelectorSafe: did not find in selector in document ' + selectors);
    }
    if (!(result instanceof HTMLElement)) {
        throw new Error(
            'documentQuerySelectorSafe: did find element in selector but it is not an html element: ' + selectors
        );
    }
    return result;
};

export const elementQuerySelectorSafe = (element: HTMLElement, selectors: string): HTMLElement => {
    const result = element.querySelector(selectors);
    if (!result) {
        throw new Error('elementQuerySelectorSafe: did not find in selector in element "' + selectors + '"');
    }
    if (!(result instanceof HTMLElement)) {
        throw new Error(
            'elementQuerySelectorSafe: did find element in selector but it is not an html element: "' + selectors + '"'
        );
    }
    return result;
};

export const getElementsByClassNameSafe = (element: HTMLElement, selectors: string): HTMLCollectionOf<Element> => {
    const result = element.getElementsByClassName(selectors);
    if (!result || result.length === 0) {
        throw new Error('getElementsByClassNameSafe: did not find in selector in element "' + selectors + '"');
    }
    return result;
};

export const makeRelease = (
    version: number,
    displayVersion: string = '',
    sourceCommitId: string = 'commit' + version,
    undeployVersion: boolean = false
): Release => ({
    version: version,
    sourceMessage: 'test' + version,
    sourceAuthor: 'test-author',
    sourceCommitId: sourceCommitId,
    displayVersion: displayVersion,
    createdAt: new Date(2002),
    undeployVersion: undeployVersion,
    prNumber: '666',
});

const date = new Date(2023, 6, 12);

export const makeLock = (input: Partial<Lock>): Lock => ({
    lockId: 'l1',
    message: 'lock msg 1',
    createdAt: date,
    createdBy: {
        name: 'default',
        email: 'default@example.com',
    },
    ...input,
});

export const makeDisplayLock = (input: Partial<DisplayLock>): DisplayLock => ({
    lockId: 'l1',
    message: 'lock msg 1',
    environment: 'default-env',
    date: date,
    // application: 'default-app', // application should not be set here, because it cannot be overwritten with undefined
    authorEmail: 'default@example.com',
    authorName: 'default',
    ...input,
});

export const fakeLoadEverything = (load: boolean): void => {
    UpdateOverview.set({
        loaded: load,
    });
    UpdateFrontendConfig.set({
        configReady: load,
    });
    updateTag.set({
        tagsReady: load,
    });
};

export const enableDexAuth = (setValidToken: boolean) => {
    if (setValidToken) {
        // Dummy token with expiring date on year 56494 
        document.cookie = 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTc3Nzd9.p3ApN5elnhhRhrh7DCOF-9suPIXYC36Nycf0nHfxuf8';
    }
    UpdateFrontendConfig.set({
        configs: {
            argoCd: undefined,
            authConfig: {
                dexAuth: {
                    enabled: true,
                },
            },
            kuberpultVersion: 'dontcare',
            manifestRepoUrl: 'dontcare',
            sourceRepoUrl: 'dontcare',
            branch: 'dontcare',
        },
    });
};
