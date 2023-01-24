/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { render, renderHook } from '@testing-library/react';
import { LocksPage } from './LocksPage';
import {
    DisplayLock,
    UpdateOverview,
    sortAppLocksFromIDs,
    sortEnvLocksFromIDs,
    useApplicationLock,
    useApplicationLockIDs,
    useEnvironmentLock,
    useEnvironmentLockIDs,
    useFilteredApplicationLockIDs,
    useFilteredEnvironmentLockIDs,
} from '../../utils/store';
import { EnvLocksTable } from '../../components/LocksTable/EnvLocksTable';
import { AppLocksTable } from '../../components/LocksTable/AppLocksTable';
import { MemoryRouter } from 'react-router-dom';
import { Environment } from '../../../api/api';

describe('LocksPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <LocksPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    it('Renders full app', () => {
        const { container } = getWrapper();
        expect(container.getElementsByClassName('mdc-data-table')[0]).toHaveTextContent('Environment Locks');
        expect(container.getElementsByClassName('mdc-data-table')[1]).toHaveTextContent('Application Locks');
    });
});

describe('Test Filter for App Locks Table', () => {
    interface dataT {
        name: string;
        envs: { [key: string]: Environment };
        query: string;
        headerTitle: string;
        columnHeaders: string[];
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <>
                <AppLocksTable {...defaultProps} {...overrides} />
            </>
        );
    };
    const getWrapper = (overrides: { lockIDs: string[]; columnHeaders: string[]; headerTitle: string }) =>
        render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'two application locks pass the filter',
            envs: {
                integration: {
                    name: 'test-env',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        testApp: {
                            name: 'testApp',
                            version: 2,
                            locks: {
                                testMessage: { message: 'testMessage', lockId: 'test-id' },
                                testMessagev2: { message: 'testMessagev2', lockId: 'another test-id' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            query: '',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(2),
        },
        {
            name: 'only one application locks passes the filter',
            envs: {
                integration: {
                    name: 'test-env',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        testApp: {
                            name: 'testApp',
                            version: 2,
                            locks: {
                                locktest: { message: 'test-message', lockId: 'test-id' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                        testAppV2: {
                            name: 'testAppV2',
                            version: 2,
                            locks: {
                                locktest: { message: 'test-message', lockId: 'another test-id' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            query: 'V2',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(1),
        },
        {
            name: 'no application lock passes the filter',
            envs: {
                integration: {
                    name: 'test-env',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        testApp: {
                            name: 'testApp',
                            version: 2,
                            locks: {
                                locktest: { message: 'test-message', lockId: 'test-id' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                        anotherTestApp: {
                            name: 'anotherTestApp',
                            version: 2,
                            locks: {
                                locktest: { message: 'test-message', lockId: 'another test-id' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            query: 'foo',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(0),
        },
    ];

    describe.each(sampleApps)(`Renders an Application Locks Table`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({ environments: testcase.envs });
            const filteredLocks = renderHook(() => useFilteredApplicationLockIDs(testcase.query)).result.current;
            const { container } = getWrapper({
                lockIDs: filteredLocks,
                columnHeaders: testcase.columnHeaders,
                headerTitle: testcase.headerTitle,
            });
            testcase.expect(container);
        });
    });
});

describe('Test Filter for Env Locks Table', () => {
    interface dataT {
        name: string;
        envs: { [key: string]: Environment };
        query: string;
        headerTitle: string;
        columnHeaders: string[];
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <>
                <EnvLocksTable {...defaultProps} {...overrides} />
            </>
        );
    };
    const getWrapper = (overrides: { lockIDs: string[]; columnHeaders: string[]; headerTitle: string }) =>
        render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'two application locks pass the filter',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {
                        testMessage: { message: 'testMessage', lockId: 'test-id' },
                        testMessagev2: { message: 'testMessagev2', lockId: 'another test-id' },
                    },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                },
            },
            query: '',
            headerTitle: 'test-title',
            columnHeaders: [
                'Date',
                'Environment',
                'Application',
                'Lock Id',
                'Message',
                'Author Name',
                'Author Email',
                '',
            ],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(2),
        },
        {
            name: 'only one application locks passes the filter',
            envs: {
                integration: {
                    name: 'integration',
                    locks: { locktest: { message: 'test-message', lockId: 'test-id' } },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                },
                development: {
                    name: 'development',
                    locks: { locktest: { message: 'test-message', lockId: 'another test-id' } },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                },
            },
            query: 'integration',
            headerTitle: 'test-title',
            columnHeaders: [
                'Date',
                'Environment',
                'Application',
                'Lock Id',
                'Message',
                'Author Name',
                'Author Email',
                '',
            ],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(1),
        },
        {
            name: 'no application lock passes the filter',
            envs: {
                integration: {
                    name: 'integration',
                    locks: { locktest: { message: 'test-message', lockId: 'test-id' } },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                },
                development: {
                    name: 'development',
                    locks: { locktest: { message: 'test-message', lockId: 'another test-id' } },
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                },
            },
            query: 'prod',
            headerTitle: 'test-title',
            columnHeaders: [
                'Date',
                'Environment',
                'Application',
                'Lock Id',
                'Message',
                'Author Name',
                'Author Email',
                '',
            ],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(0),
        },
    ];

    describe.each(sampleApps)(`Renders an Environment Locks Table`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({ environments: testcase.envs });
            const filteredLocks = renderHook(() => useFilteredEnvironmentLockIDs(testcase.query)).result.current;
            const { container } = getWrapper({
                lockIDs: filteredLocks,
                columnHeaders: testcase.columnHeaders,
                headerTitle: testcase.headerTitle,
            });
            testcase.expect(container);
        });
    });
});

describe('Test env locks', () => {
    interface dataEnvT {
        name: string;
        envs: { [key: string]: Environment };
        sortOrder: 'ascending' | 'descending';
        expectedLockIDs: string[];
    }

    const sampleEnvData: dataEnvT[] = [
        {
            name: 'no locks',
            envs: {},
            sortOrder: 'ascending',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: {
                integration: {
                    name: 'integration',
                    locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            },
            sortOrder: 'ascending',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get a few locks (sorted, descending)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            },
            sortOrder: 'descending',
            expectedLockIDs: ['ui-v2-1337', 'ui-v2-123', 'ui-v2-321'],
        },
        {
            name: 'get a few locks (sorted, ascending)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 2,
                },
            },
            sortOrder: 'ascending',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-123', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleEnvData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
            // when
            const obtained = renderHook(() => useEnvironmentLockIDs()).result.current;
            // then
            const sortedObtained = renderHook(() => sortEnvLocksFromIDs(obtained, testcase.sortOrder)).result.current;
            expect(sortedObtained).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataEnvFilteredT {
        name: string;
        envs: { [key: string]: Environment };
        filter: string;
        expectedLockIDs: string[];
    }

    const sampleFilteredEnvData: dataEnvFilteredT[] = [
        {
            name: 'no locks',
            envs: {},
            filter: 'integration',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: {
                integration: {
                    name: 'integration',
                    locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            },
            filter: 'integration',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get filtered locks (integration, get 1 lock)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
                development: {
                    name: 'development',
                    locks: {
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            },
            filter: 'integration',
            expectedLockIDs: ['ui-v2-123'],
        },
        {
            name: 'get filtered locks (development, get 2 lock)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {
                        lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
                development: {
                    name: 'development',
                    locks: {
                        locktest: { message: 'locktest', lockId: 'ui-v2-1337', createdAt: new Date(1995, 11, 17) },
                        lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                    },
                    applications: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            },
            filter: 'development',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleFilteredEnvData)(`Test Filtered Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
            // when
            const obtained = renderHook(() => useFilteredEnvironmentLockIDs(testcase.filter)).result.current;
            // then
            const sortedObtained = renderHook(() => sortEnvLocksFromIDs(obtained, 'ascending')).result.current;
            expect(sortedObtained).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataTranslateEnvLockT {
        name: string;
        envs: { [key: string]: Environment };
        id: string;
        expectedLock: DisplayLock;
    }

    const sampleTranslateEnvLockData: dataTranslateEnvLockT[] = [
        {
            name: 'Translate lockID to DisplayLock',
            envs: {
                integration: {
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
            },
            id: 'ui-v2-1337',
            expectedLock: {
                date: new Date(1995, 11, 17),
                lockId: 'ui-v2-1337',
                environment: 'integration',
                message: 'locktest',
                authorEmail: 'kuberpult@fdc.com',
                authorName: 'kuberpultUser',
            } as DisplayLock,
        },
    ];

    describe.each(sampleTranslateEnvLockData)(`Test translating lockID to DisplayLock`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
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
        envs: { [key: string]: Environment };
        sortOrder: 'ascending' | 'descending';
        expectedLockIDs: string[];
    }

    const sampleAppData: dataAppT[] = [
        {
            name: 'no locks',
            envs: {},
            sortOrder: 'ascending',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: { locktest: { message: 'locktest', lockId: 'ui-v2-1337' } },
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            },
            sortOrder: 'ascending',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get a few locks (sorted, descending)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 1337,
                            locks: {
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                            queuedVersion: 0,
                            undeployVersion: true,
                        },
                    },
                },
            },
            sortOrder: 'descending',
            expectedLockIDs: ['ui-v2-1337', 'ui-v2-123', 'ui-v2-321'],
        },
        {
            name: 'get a few locks (sorted, ascending)',
            envs: {
                integration: {
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
                        },
                    },
                },
            },
            sortOrder: 'ascending',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-123', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleAppData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
            // when
            const obtained = renderHook(() => useApplicationLockIDs()).result.current;
            // then
            const sortedObtained = renderHook(() => sortAppLocksFromIDs(obtained, testcase.sortOrder)).result.current;
            expect(sortedObtained).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataAppFilteredT {
        name: string;
        envs: { [key: string]: Environment };
        filter: string;
        expectedLockIDs: string[];
    }

    const sampleAppFilteredData: dataAppFilteredT[] = [
        {
            name: 'no locks',
            envs: {},
            filter: 'integration',
            expectedLockIDs: [],
        },
        {
            name: 'get one lock',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 2,
                            locks: {
                                locktest: { message: 'locktest', lockId: 'ui-v2-1337' },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            filter: 'foo',
            expectedLockIDs: ['ui-v2-1337'],
        },
        {
            name: 'get filtered locks (integration, get 1 lock)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 2,
                            locks: {
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                            },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
                development: {
                    name: 'development',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        bar: {
                            name: 'bar',
                            version: 1337,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                        },
                    },
                },
            },
            filter: 'foo',
            expectedLockIDs: ['ui-v2-123'],
        },
        {
            name: 'get filtered locks (development, get 2 lock)',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            version: 420,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                lockfoo: { message: 'lockfoo', lockId: 'ui-v2-123', createdAt: new Date(1995, 11, 16) },
                            },
                        },
                    },
                },
                development: {
                    name: 'development',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        bar: {
                            name: 'bar',
                            version: 1337,
                            queuedVersion: 0,
                            undeployVersion: false,
                            locks: {
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                },
                                lockbar: { message: 'lockbar', lockId: 'ui-v2-321', createdAt: new Date(1995, 11, 15) },
                            },
                        },
                    },
                },
            },
            filter: 'bar',
            expectedLockIDs: ['ui-v2-321', 'ui-v2-1337'],
        },
    ];

    describe.each(sampleAppFilteredData)(`Test Filtered Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
            // when
            const obtained = renderHook(() => useFilteredApplicationLockIDs(testcase.filter)).result.current;
            // then
            const sortedObtained = renderHook(() => sortAppLocksFromIDs(obtained, 'ascending')).result.current;
            expect(sortedObtained).toStrictEqual(testcase.expectedLockIDs);
        });
    });

    interface dataTranslateAppLockT {
        name: string;
        envs: { [key: string]: Environment };
        id: string;
        expectedLock: DisplayLock;
    }

    const sampleTranslateAppLockData: dataTranslateAppLockT[] = [
        {
            name: 'Translate lockID to DisplayLock',
            envs: {
                integration: {
                    name: 'integration',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        foo: {
                            name: 'foo',
                            queuedVersion: 0,
                            undeployVersion: false,
                            version: 1337,
                            locks: {
                                locktest: {
                                    message: 'locktest',
                                    lockId: 'ui-v2-1337',
                                    createdAt: new Date(1995, 11, 17),
                                    createdBy: { email: 'kuberpult@fdc.com', name: 'kuberpultUser' },
                                },
                            },
                        },
                    },
                },
            },
            id: 'ui-v2-1337',
            expectedLock: {
                application: 'foo',
                date: new Date(1995, 11, 17),
                lockId: 'ui-v2-1337',
                environment: 'integration',
                message: 'locktest',
                authorEmail: 'kuberpult@fdc.com',
                authorName: 'kuberpultUser',
            } as DisplayLock,
        },
    ];

    describe.each(sampleTranslateAppLockData)(`Test translating lockID to DisplayLock`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ environments: testcase.envs });
            // when
            const obtained = renderHook(() => useApplicationLock(testcase.id)).result.current;
            // then
            expect(obtained).toStrictEqual(testcase.expectedLock);
        });
    });
});
