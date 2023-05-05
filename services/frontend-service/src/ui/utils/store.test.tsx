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

Copyright 2023 freiheit.com*/
import { renderHook } from '@testing-library/react';
import { AllLocks, updateActions, UpdateOverview, useLocksSimilarTo } from './store';
import { BatchAction, EnvironmentGroup, Priority } from '../../api/api';
import { makeDisplayLock, makeLock } from '../../setupTests';

type TestDataStore = {
    name: string;
    inputEnvGroups: EnvironmentGroup[]; // this just defines what locks generally exist
    inputAction: BatchAction; // the action we are rendering currently in the sidebar
    expectedLocks: AllLocks;
};

const testdata: TestDataStore[] = [
    {
        name: 'empty data',
        inputAction: {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: 'dev',
                    lockId: 'l1',
                },
            },
        },
        inputEnvGroups: [],
        expectedLocks: {
            appLocks: [],
            environmentLocks: [],
        },
    },
    {
        name: 'one env lock: should not find another lock',
        inputAction: {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: 'dev',
                    lockId: 'l1',
                },
            },
        },
        inputEnvGroups: [
            {
                environments: [
                    {
                        name: 'dev',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {},
                    },
                ],
                environmentGroupName: 'group1',
                distanceToUpstream: 0,
            },
        ],
        expectedLocks: {
            appLocks: [],
            environmentLocks: [],
        },
    },
    {
        name: 'two env locks with same ID on different envs: should find the other lock',
        inputAction: {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: 'dev',
                    lockId: 'l1',
                },
            },
        },
        inputEnvGroups: [
            {
                environments: [
                    {
                        name: 'dev',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {},
                    },
                    {
                        name: 'staging',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {},
                    },
                ],
                environmentGroupName: 'group1',
                distanceToUpstream: 0,
            },
        ],
        expectedLocks: {
            appLocks: [],
            environmentLocks: [
                makeDisplayLock({
                    lockId: 'l1',
                    environment: 'staging',
                    authorName: 'Betty',
                    authorEmail: 'betty@example.com',
                }),
            ],
        },
    },
    {
        name: 'env lock and app lock with same ID: deleting the env lock should find the other lock',
        inputAction: {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: 'dev',
                    lockId: 'l1',
                },
            },
        },
        inputEnvGroups: [
            {
                environments: [
                    {
                        name: 'dev',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {
                            app1: {
                                name: 'betty',
                                locks: {
                                    l1: makeLock({ lockId: 'l1' }),
                                },
                                version: 666,
                                undeployVersion: false,
                                queuedVersion: 0,
                                argoCD: undefined,
                            },
                        },
                    },
                ],
                environmentGroupName: 'group1',
                distanceToUpstream: 0,
            },
        ],
        expectedLocks: {
            appLocks: [
                makeDisplayLock({
                    environment: 'dev',
                    lockId: 'l1',
                    application: 'betty',
                    message: 'lock msg 1',
                    authorName: 'Betty',
                    authorEmail: 'betty@example.com',
                }),
            ],
            environmentLocks: [],
        },
    },
    {
        name: 'bug: delete-all button must appear for each entry. 2 Env Locks, 1 App lock exist. 1 Env lock, 1 app lock in cart',
        inputAction: {
            action: {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: {
                    environment: 'dev',
                    lockId: 'l1',
                    application: 'app1',
                },
            },
        },
        inputEnvGroups: [
            {
                environments: [
                    {
                        name: 'dev',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {
                            app1: {
                                name: 'betty',
                                locks: {
                                    l1: makeLock({ lockId: 'l1' }),
                                },
                                version: 666,
                                undeployVersion: false,
                                queuedVersion: 0,
                                argoCD: undefined,
                            },
                        },
                    },
                    {
                        name: 'dev2',
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                        locks: {
                            l1: makeLock({ lockId: 'l1' }),
                        },
                        applications: {},
                    },
                ],
                environmentGroupName: 'group1',
                distanceToUpstream: 0,
            },
        ],
        expectedLocks: {
            appLocks: [
                makeDisplayLock({
                    environment: 'dev',
                    lockId: 'l1',
                    application: 'betty',
                    message: 'lock msg 1',
                    authorName: 'Betty',
                    authorEmail: 'betty@example.com',
                }),
            ],
            environmentLocks: [
                makeDisplayLock({
                    environment: 'dev',
                    lockId: 'l1',
                    message: 'lock msg 1',
                    authorName: 'Betty',
                    authorEmail: 'betty@example.com',
                }),
                makeDisplayLock({
                    environment: 'dev2',
                    lockId: 'l1',
                    message: 'lock msg 1',
                    authorName: 'Betty',
                    authorEmail: 'betty@example.com',
                }),
            ],
        },
    },
];

describe.each(testdata)('Test useLocksSimilarTo', (testcase) => {
    it(testcase.name, () => {
        // given
        updateActions([]);
        UpdateOverview.set({
            applications: {},
            environmentGroups: testcase.inputEnvGroups,
        });
        // when
        const actions = renderHook(() => useLocksSimilarTo(testcase.inputAction)).result.current;
        // then
        expect(actions.appLocks).toStrictEqual(testcase.expectedLocks.appLocks);
        expect(actions.environmentLocks).toStrictEqual(testcase.expectedLocks.environmentLocks);
    });
});
