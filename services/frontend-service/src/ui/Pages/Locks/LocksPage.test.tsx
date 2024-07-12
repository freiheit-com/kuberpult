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
import { render, renderHook } from '@testing-library/react';
import { LocksPage } from './LocksPage';
import {
    DisplayLock,
    UpdateOverview,
    useAllLocks,
    useEnvironmentLock,
    useFilteredEnvironmentLockIDs,
} from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';
import { Environment, Priority } from '../../../api/api';
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';

describe('LocksPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <LocksPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    it('Renders full app', () => {
        fakeLoadEverything(true);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('mdc-data-table')[0]).toHaveTextContent('Environment Locks');
        expect(container.getElementsByClassName('mdc-data-table')[1]).toHaveTextContent('Application Locks');
    });
    it('Renders login page if Dex enabled', () => {
        fakeLoadEverything(true);
        enableDexAuth(false);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('environment_name')[0]).toHaveTextContent('Log in to Dex');
    });
    it('Renders page page if Dex enabled and valid token', () => {
        fakeLoadEverything(true);
        enableDexAuth(true);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('mdc-data-table')[0]).toHaveTextContent('Environment Locks');
        expect(container.getElementsByClassName('mdc-data-table')[1]).toHaveTextContent('Application Locks');
    });
    it('Renders spinner', () => {
        // given
        UpdateOverview.set({
            loaded: false,
        });
        // when
        const { container } = getWrapper();
        // then
        expect(container.getElementsByClassName('spinner')).toHaveLength(1);
    });
});

describe('Test env locks', () => {
    interface dataEnvT {
        name: string;
        envs: Environment[];
        sortOrder: 'oldestToNewest' | 'newestToOldest';
        expectedLockIDs: string[];
    }

    const sampleEnvData: dataEnvT[] = [
        {
            name: 'no locks',
            envs: [],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: [
                {
                    name: 'integration',
                    locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get a few locks (sorted, newestToOldest)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            ],
            sortOrder: 'newestToOldest',
            expectedLockIDs: ['ui-v2-1337', 'ui-v2-123', 'ui-v2-321'],
        },
        {
            name: 'get a few locks (sorted, oldestToNewest)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-123', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleEnvData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'dontcare',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            // when
            const obtained = renderHook(() => useAllLocks().environmentLocks).result.current;
            // then
            expect(obtained.map((lock) => lock.lockId)).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataEnvFilteredT {
        name: string;
        envs: Environment[];
        filter: string;
        expectedLockIDs: string[];
    }

    const sampleFilteredEnvData: dataEnvFilteredT[] = [
        {
            name: 'no locks',
            envs: [],
            filter: 'integration',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: [
                {
                    name: 'integration',
                    locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            filter: 'integration',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get filtered locks (integration, get 1 lock)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
                {
                    name: 'development',
                    locks: {
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            filter: 'integration',
            expectedLockIDs: ['ui-v2-123'],
        },
        {
            name: 'get filtered locks (development, get 2 lock)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
                {
                    name: 'development',
                    locks: {
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            filter: 'development',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleFilteredEnvData)(`Test Filtered Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                // environments: testcase.envs,
                environmentGroups: [
                    {
                        distanceToUpstream: 0,
                        environmentGroupName: 'group1',
                        environments: testcase.envs,
                        priority: Priority.YOLO,
                    },
                ],
            });
            // when
            const obtained = renderHook(() => useFilteredEnvironmentLockIDs(testcase.filter)).result.current;
            // then
            expect(obtained).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataTranslateEnvLockT {
        name: string;
        envs: [Environment];
        id: string;
        expectedLock: DisplayLock;
    }

    const sampleTranslateEnvLockData: dataTranslateEnvLockT[] = [
        {
            name: 'Translate lockID to DisplayLock',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        locktest: {
                            message: 'locktest',
                            lockId: 'ui-v2-1337',
                            createdAt: new Date(1995, 11, 17),
                            createdBy: { email: 'kuberpult@fdc.com', name: 'kuberpultUser' },
                        },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            id: 'ui-v2-1337',
            expectedLock: {
                date: new Date(1995, 11, 17),
                lockId: 'ui-v2-1337',
                environment: 'integration',
                message: 'locktest',
                authorEmail: 'kuberpult@fdc.com',
                authorName: 'kuberpultUser',
            },
        },
    ];

    describe.each(sampleTranslateEnvLockData)(`Test translating lockID to DisplayLock`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                applications: {},
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            // when
            const obtained = renderHook(() => useEnvironmentLock(testcase.id)).result.current;
            // then
            expect(obtained).toStrictEqual(testcase.expectedLock);
        });
    });
});

describe('Test app locks', () => {
    interface dataAppT {
        name: string;
        envs: Environment[];
        sortOrder: 'oldestToNewest' | 'newestToOldest';
        expectedLockIDs: string[];
    }

    const sampleAppData: dataAppT[] = [
        {
            name: 'no locks',
            envs: [],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: [
                {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get a few locks (sorted, newestToOldest)',
            envs: [
                {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: {
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            ],
            sortOrder: 'newestToOldest',
            expectedLockIDs: ['ui-v2-1337', 'ui-v2-123', 'ui-v2-321'],
        },
        {
            name: 'get a few locks (sorted, oldestToNewest)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                            teamLocks: {},
                            team: 'test-team',
                        },
                        bar: {
                            name: 'bar',
                            version: 420,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                            },
                            teamLocks: {},
                            team: 'test-team',
                        },
                    },
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-123', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleAppData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            // UpdateOverview.set({ environmentGroups: testcase.envs });
            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'dontcare',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            // when
            const obtained = renderHook(() => useAllLocks().appLocks).result.current;
            // then
            expect(obtained.map((lock) => lock.lockId)).toStrictEqual(testcase.expectedLockIDs);
        });
    });
});

describe('Test Team locks', () => {
    interface dataAppT {
        name: string;
        envs: Environment[];
        sortOrder: 'oldestToNewest' | 'newestToOldest';
        expectedLockIDs: string[];
    }

    const sampleAppData: dataAppT[] = [
        {
            name: 'no locks',
            envs: [],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: [
                {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: {},
                            teamLocks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get a few locks (sorted, newestToOldest)',
            envs: [
                {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: {},
                            teamLocks: {
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            ],
            sortOrder: 'newestToOldest',
            expectedLockIDs: ['ui-v2-1337', 'ui-v2-123', 'ui-v2-321'],
        },
        {
            name: 'get a few locks (sorted, oldestToNewest)',
            envs: [
                {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                            teamLocks: {
                                lockbar: {
                                    message: 'team lock 1',
                                    lockId: 'ui-v2-t-lock-1',
                                    createdAt: new Date(1995, 11, 15),
                                },
                            },
                            team: 'test-team',
                        },
                        bar: {
                            name: 'bar',
                            version: 420,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                            },
                            teamLocks: {
                                lockbar: {
                                    message: 'team lock 2',
                                    lockId: 'ui-v2-t-lock-2',
                                    createdAt: new Date(1995, 11, 15),
                                },
                            },
                            team: 'test-team',
                        },
                    },
                },
            ],
            sortOrder: 'oldestToNewest',
            expectedLockIDs: ['ui-v2-t-lock-1', 'ui-v2-t-lock-2'],
        },
    ];

    describe.each(sampleAppData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'dontcare',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            // when
            const obtained = renderHook(() => useAllLocks().teamLocks).result.current;
            // then
            expect(obtained.map((lock) => lock.lockId)).toStrictEqual(testcase.expectedLockIDs);
        });
    });
});
