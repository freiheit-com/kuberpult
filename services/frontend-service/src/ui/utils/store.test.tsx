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
import { act, renderHook } from '@testing-library/react';
import {
    addAction,
    AllLocks,
    AppDetailsResponse,
    AppDetailsState,
    appendAction,
    DisplayLock,
    FlushRolloutStatus,
    SnackbarStatus,
    UpdateAction,
    updateActions,
    UpdateAllApplicationLocks,
    updateAppDetails,
    UpdateOverview,
    UpdateRolloutStatus,
    UpdateSnackbar,
    useLocksConflictingWithActions,
    useLocksSimilarTo,
    useNavigateWithSearchParams,
    useReleaseDifference,
    useRolloutStatus,
} from './store';
import {
    AllAppLocks,
    BatchAction,
    Environment,
    EnvironmentGroup,
    GetOverviewResponse,
    LockBehavior,
    OverviewApplication,
    Priority,
    ReleaseTrainRequest_TargetType,
    RolloutStatus,
    StreamStatusResponse,
    UndeploySummary,
} from '../../api/api';
import { makeDisplayLock, makeLock } from '../../setupTests';
import { BrowserRouter } from 'react-router-dom';
import { ReactNode } from 'react';

describe('Test useLocksSimilarTo', () => {
    type TestDataStore = {
        name: string;
        inputEnvGroups: EnvironmentGroup[]; // this just defines what locks generally exist
        inputAction: BatchAction; // the action we are rendering currently in the sidebar
        expectedLocks: AllLocks;
        OverviewApps: OverviewApplication[];
        AppLocks: { [key: string]: AllAppLocks };
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
            OverviewApps: [],
            AppLocks: {},
            inputEnvGroups: [],
            expectedLocks: {
                appLocks: [],
                environmentLocks: [],
                teamLocks: [],
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
            OverviewApps: [],
            AppLocks: {},
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
                            teamLocks: {},
                        },
                    ],
                    environmentGroupName: 'group1',
                    distanceToUpstream: 0,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
            expectedLocks: {
                appLocks: [],
                environmentLocks: [],
                teamLocks: [],
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
            OverviewApps: [],
            AppLocks: {},
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
                            teamLocks: {},
                        },
                        {
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.UPSTREAM,
                            locks: {
                                l1: makeLock({ lockId: 'l1' }),
                            },
                            teamLocks: {},
                        },
                    ],
                    environmentGroupName: 'group1',
                    distanceToUpstream: 0,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
            expectedLocks: {
                appLocks: [],
                environmentLocks: [
                    makeDisplayLock({
                        lockId: 'l1',
                        environment: 'staging',
                    }),
                ],
                teamLocks: [],
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
            AppLocks: {
                dev: {
                    appLocks: {
                        betty: {
                            locks: [makeLock({ lockId: 'l1' })],
                        },
                    },
                },
            },
            OverviewApps: [{ name: 'betty', team: '' }],
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
                            teamLocks: {},
                        },
                    ],
                    environmentGroupName: 'group1',
                    distanceToUpstream: 0,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
            expectedLocks: {
                appLocks: [
                    makeDisplayLock({
                        environment: 'dev',
                        lockId: 'l1',
                        application: 'betty',
                        message: 'lock msg 1',
                    }),
                ],
                environmentLocks: [],
                teamLocks: [],
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
            OverviewApps: [
                {
                    name: 'betty',
                    team: 'test-team',
                },
            ],
            AppLocks: {
                dev: {
                    appLocks: {
                        betty: {
                            locks: [makeLock({ lockId: 'l1' })],
                        },
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
                            teamLocks: {
                                'test-team': {
                                    locks: [makeLock({ lockId: 'l1' })],
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
                            teamLocks: {},
                        },
                    ],
                    environmentGroupName: 'group1',
                    distanceToUpstream: 0,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
            expectedLocks: {
                appLocks: [
                    makeDisplayLock({
                        environment: 'dev',
                        lockId: 'l1',
                        application: 'betty',
                        message: 'lock msg 1',
                    }),
                ],
                environmentLocks: [
                    makeDisplayLock({
                        environment: 'dev',
                        lockId: 'l1',
                        message: 'lock msg 1',
                    }),
                    makeDisplayLock({
                        environment: 'dev2',
                        lockId: 'l1',
                        message: 'lock msg 1',
                    }),
                ],
                teamLocks: [
                    makeDisplayLock({
                        environment: 'dev',
                        lockId: 'l1',
                        team: 'test-team',
                        message: 'lock msg 1',
                    }),
                ],
            },
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions([]);
            UpdateOverview.set({
                lightweightApps: testcase.OverviewApps,
                environmentGroups: testcase.inputEnvGroups,
            });
            UpdateAllApplicationLocks.set(testcase.AppLocks);
            // when
            const actions = renderHook(() => useLocksSimilarTo(testcase.inputAction)).result.current;
            // then
            expect(actions.appLocks).toStrictEqual(testcase.expectedLocks.appLocks);
            expect(actions.environmentLocks).toStrictEqual(testcase.expectedLocks.environmentLocks);
            expect(actions.teamLocks).toStrictEqual(testcase.expectedLocks.teamLocks);
        });
    });
});

describe('Test useNavigateWithSearchParams', () => {
    type TestDataStore = {
        name: string;
        currentURL: string;
        navigationTo: string;
        expectedURL: string;
    };

    const testdata: TestDataStore[] = [
        {
            name: 'url with no search parameters',
            currentURL: '',
            navigationTo: 'test-random-page',
            expectedURL: 'test-random-page',
        },
        {
            name: 'url with some search parameters',
            currentURL: '/ui/home/test/whaat?query1=boo&query2=bar',
            navigationTo: 'test-random-page',
            expectedURL: 'test-random-page?query1=boo&query2=bar',
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            const wrapper = ({ children }: { children: ReactNode }) => <BrowserRouter>{children}</BrowserRouter>;
            window.history.pushState({}, 'Test page', testcase.currentURL);
            // when
            const result = renderHook(() => useNavigateWithSearchParams(testcase.navigationTo), { wrapper }).result
                .current;
            // then
            expect(result.navURL).toBe(testcase.expectedURL);
            // when
            act(() => {
                result.navCallback();
            });
            // then
            expect(window.location.href).toContain(testcase.expectedURL);
        });
    });
});

describe('Rollout Status', () => {
    type Testcase = {
        name: string;
        events: Array<StreamStatusResponse | { error: true }>;
        expectedApps: Array<{
            application: string;
            environment: string;
            version: number;
            rolloutStatus: RolloutStatus | undefined;
        }>;
    };

    const testdata: Array<Testcase> = [
        {
            name: 'not enabled if empty',
            events: [],

            expectedApps: [
                {
                    application: 'app1',
                    environment: 'env1',
                    version: 0,
                    rolloutStatus: undefined,
                },
            ],
        },
        {
            name: 'updates one app',
            events: [
                {
                    environment: 'env1',
                    application: 'app1',
                    version: 1,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
            ],

            expectedApps: [
                {
                    application: 'app1',
                    environment: 'env1',
                    version: 1,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
            ],
        },
        {
            name: 'keep the latest entry per app and environment',
            events: [
                {
                    environment: 'env1',
                    application: 'app1',
                    version: 1,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'env1',
                    application: 'app1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
            ],

            expectedApps: [
                {
                    application: 'app1',
                    environment: 'env1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
            ],
        },
        {
            name: 'flushes the state',
            events: [
                {
                    environment: 'env1',
                    application: 'app1',
                    version: 1,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                { error: true },
            ],

            expectedApps: [
                {
                    application: 'app1',
                    environment: 'env1',
                    version: 0,
                    rolloutStatus: undefined,
                },
            ],
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            FlushRolloutStatus();
            testcase.events.forEach((ev) => {
                if ('error' in ev) {
                    FlushRolloutStatus();
                } else {
                    UpdateRolloutStatus(ev);
                }
            });
            testcase.expectedApps.forEach((app) => {
                const rollout = renderHook(() =>
                    useRolloutStatus((getter) => getter.getAppStatus(app.application, app.version, app.environment))
                );
                expect(rollout.result.current).toEqual(app.rolloutStatus);
            });
        });
    });
});

describe('Test addAction duplicate detection', () => {
    type TestDataStore = {
        name: string;
        firstAction: BatchAction;
        differentAction: BatchAction;
    };

    const testdata: TestDataStore[] = [
        {
            name: 'create environment lock',
            firstAction: {
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: {
                        environment: 'dev',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: {
                        environment: 'staging',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
        },
        {
            name: 'delete environment lock',
            firstAction: {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: 'dev',
                        lockId: 'foo',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: 'staging',
                        lockId: 'foo',
                    },
                },
            },
        },
        {
            name: 'create app lock',
            firstAction: {
                action: {
                    $case: 'createEnvironmentApplicationLock',
                    createEnvironmentApplicationLock: {
                        environment: 'dev',
                        application: 'app1',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'createEnvironmentApplicationLock',
                    createEnvironmentApplicationLock: {
                        environment: 'dev',
                        application: 'app2',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
        },
        {
            name: 'delete app lock',
            firstAction: {
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        environment: 'dev',
                        application: 'app1',
                        lockId: 'foo',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        environment: 'dev',
                        application: 'app2',
                        lockId: 'foo',
                    },
                },
            },
        },
        {
            name: 'create team lock',
            firstAction: {
                action: {
                    $case: 'createEnvironmentTeamLock',
                    createEnvironmentTeamLock: {
                        environment: 'dev',
                        team: 'team1',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'createEnvironmentTeamLock',
                    createEnvironmentTeamLock: {
                        environment: 'dev',
                        team: 'team2',
                        lockId: 'foo',
                        message: 'do it',
                        ciLink: '',
                    },
                },
            },
        },
        {
            name: 'delete team lock',
            firstAction: {
                action: {
                    $case: 'deleteEnvironmentTeamLock',
                    deleteEnvironmentTeamLock: {
                        environment: 'dev',
                        team: 'team1',
                        lockId: 'foo',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deleteEnvironmentTeamLock',
                    deleteEnvironmentTeamLock: {
                        environment: 'dev',
                        team: 'team2',
                        lockId: 'foo',
                    },
                },
            },
        },
        {
            name: 'deploy',
            firstAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app1',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app2',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
        },
        {
            name: 'undeploy',
            firstAction: {
                action: {
                    $case: 'undeploy',
                    undeploy: {
                        application: 'app1',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'undeploy',
                    undeploy: {
                        application: 'app2',
                    },
                },
            },
        },
        {
            name: 'prepare undeploy',
            firstAction: {
                action: {
                    $case: 'prepareUndeploy',
                    prepareUndeploy: {
                        application: 'app1',
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'prepareUndeploy',
                    prepareUndeploy: {
                        application: 'app2',
                    },
                },
            },
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions([]);
            UpdateSnackbar.set({ show: false, status: SnackbarStatus.SUCCESS, content: '' });

            expect(UpdateSnackbar.get().show).toStrictEqual(false);
            // when
            addAction(testcase.firstAction);
            expect(UpdateSnackbar.get().show).toStrictEqual(false);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(1);

            // when
            addAction(testcase.firstAction);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(1);
            //and
            expect(UpdateSnackbar.get().show).toStrictEqual(true);

            // when
            addAction(testcase.differentAction);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(2);
            // when
            addAction(testcase.differentAction);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(2);
        });
    });
});

describe('Test maxActions', () => {
    type TestDataStore = {
        name: string;
        inputActionsLen: number;
        expectedLen: number;
        expectedShowError: boolean;
    };

    const testdata: TestDataStore[] = [
        {
            name: 'below limit',
            inputActionsLen: 99,
            expectedLen: 99,
            expectedShowError: false,
        },
        {
            name: 'at limit',
            inputActionsLen: 100,
            expectedLen: 100,
            expectedShowError: false,
        },
        {
            name: 'over limit',
            inputActionsLen: 101,
            expectedLen: 100,
            expectedShowError: true,
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions([]);
            //and
            UpdateSnackbar.set({ show: false, status: SnackbarStatus.SUCCESS, content: '' });
            // when
            for (let i = 0; i < testcase.inputActionsLen; i++) {
                appendAction([
                    {
                        action: {
                            $case: 'deploy',
                            deploy: {
                                environment: 'foo',
                                application: 'bread' + i,
                                version: i,
                                ignoreAllLocks: false,
                                lockBehavior: LockBehavior.IGNORE,
                            },
                        },
                    },
                ]);
            }

            // then
            expect(UpdateSnackbar.get().show).toStrictEqual(testcase.expectedShowError);
            expect(UpdateAction.get().actions.length).toStrictEqual(testcase.expectedLen);
        });
    });
});

describe('Test useLocksConflictingWithActions', () => {
    type TestDataStore = {
        name: string;
        actions: BatchAction[];
        expectedAppLocks: DisplayLock[];
        expectedEnvLocks: DisplayLock[];
        environments: Environment[];
        OverviewApps: OverviewApplication[];
        AppLocks: {
            [key: string]: AllAppLocks;
        };
    };

    const testdata: TestDataStore[] = [
        {
            name: 'empty actions empty locks',
            actions: [],
            expectedAppLocks: [],
            expectedEnvLocks: [],
            environments: [],
            OverviewApps: [],
            AppLocks: {},
        },
        {
            name: 'deploy action and related app lock and env lock',
            actions: [
                {
                    action: {
                        $case: 'deploy',
                        deploy: {
                            environment: 'dev',
                            application: 'app1',
                            version: 1,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.IGNORE,
                        },
                    },
                },
            ],
            OverviewApps: [
                {
                    name: 'app1',
                    team: '',
                },
            ],
            environments: [
                {
                    name: 'dev',
                    locks: {
                        'lock-env-dev': makeLock({
                            message: 'locked because christmas',
                            lockId: 'my-env-lock1',
                        }),
                    },
                    teamLocks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            AppLocks: {
                dev: {
                    appLocks: {
                        app1: {
                            locks: [
                                makeLock({
                                    lockId: 'app-lock-id',
                                    message: 'i do not like this app',
                                }),
                            ],
                        },
                    },
                },
            },
            expectedAppLocks: [
                makeDisplayLock({
                    lockId: 'app-lock-id',
                    application: 'app1',
                    message: 'i do not like this app',
                    environment: 'dev',
                }),
            ],
            expectedEnvLocks: [
                makeDisplayLock({
                    lockId: 'my-env-lock1',
                    environment: 'dev',
                    message: 'locked because christmas',
                }),
            ],
        },
        {
            name: 'deploy action and unrelated locks',
            OverviewApps: [
                {
                    name: 'anotherapp',
                    team: '',
                },
            ],
            actions: [
                {
                    action: {
                        $case: 'deploy',
                        deploy: {
                            environment: 'dev',
                            application: 'app2',
                            version: 1,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.IGNORE,
                        },
                    },
                },
            ],
            environments: [
                {
                    name: 'staging', // this lock differs by stage
                    locks: {
                        'lock-env-dev': makeLock({
                            message: 'locked because christmas',
                            lockId: 'my-env-lock1',
                        }),
                    },
                    teamLocks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            AppLocks: {
                staging: {
                    appLocks: {
                        anotherapp: {
                            locks: [
                                makeLock({
                                    lockId: 'app-lock-id',
                                    message: 'i do not like this app',
                                }),
                            ],
                        },
                    },
                },
            },
            expectedAppLocks: [],
            expectedEnvLocks: [],
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions(testcase.actions);
            UpdateOverview.set({
                lightweightApps: testcase.OverviewApps,
                environmentGroups: [
                    {
                        environmentGroupName: 'g1',
                        environments: testcase.environments,
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            UpdateAllApplicationLocks.set(testcase.AppLocks);
            // when
            const actualLocks = renderHook(() => useLocksConflictingWithActions()).result.current;
            // then
            expect(actualLocks.environmentLocks).toStrictEqual(testcase.expectedEnvLocks);
            expect(actualLocks.appLocks).toStrictEqual(testcase.expectedAppLocks);
        });
    });
});

describe('Test addAction blocking release train additions', () => {
    type TestDataStore = {
        name: string;
        firstAction: BatchAction;
        differentAction: BatchAction;
        expectedActions: number;
    };

    const testdata: TestDataStore[] = [
        {
            name: 'deploy 2 in a row',
            expectedActions: 2,
            firstAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app1',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app2',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
        },
        {
            name: 'can not add release train after deploy action',
            expectedActions: 1,
            firstAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app1',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'releaseTrain',
                    releaseTrain: {
                        target: 'dev',
                        team: '',
                        commitHash: '',
                        ciLink: '',
                        targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                    },
                },
            },
        },
        {
            name: 'can not add release train after release train',
            expectedActions: 1,
            firstAction: {
                action: {
                    $case: 'releaseTrain',
                    releaseTrain: {
                        target: 'dev',
                        team: '',
                        commitHash: '',
                        ciLink: '',
                        targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'releaseTrain',
                    releaseTrain: {
                        target: 'stagin',
                        team: '',
                        commitHash: '',
                        ciLink: '',
                        targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                    },
                },
            },
        },
        {
            name: 'can not add deploy action after release train',
            expectedActions: 1,
            firstAction: {
                action: {
                    $case: 'releaseTrain',
                    releaseTrain: {
                        target: 'dev',
                        team: '',
                        commitHash: '',
                        ciLink: '',
                        targetType: ReleaseTrainRequest_TargetType.UNKNOWN,
                    },
                },
            },
            differentAction: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'dev',
                        application: 'app1',
                        version: 1,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
        },
    ];

    describe.each(testdata)('with', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions([]);

            // when
            addAction(testcase.firstAction);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(1);

            // when
            addAction(testcase.differentAction);
            // then
            expect(UpdateAction.get().actions.length).toStrictEqual(testcase.expectedActions);
        });
    });
});

describe('Test Calculate Release Difference', () => {
    type TestDataStore = {
        name: string;
        inputOverview: GetOverviewResponse;
        inputAppDetails: { [p: string]: AppDetailsResponse };
        inputVersion: number;
        expectedDifference: number;
    };

    const appName = 'differentApp';
    const envName = 'testEnv';

    const testdata: TestDataStore[] = [
        {
            name: 'app does not exist in the app Details',
            inputAppDetails: {},
            inputOverview: {
                environmentGroups: [
                    {
                        environmentGroupName: 'test',
                        environments: [
                            {
                                name: envName,
                                locks: {},
                                teamLocks: {},
                                distanceToUpstream: 0,
                                priority: Priority.PROD,
                            },
                        ],
                        distanceToUpstream: 0,
                        priority: Priority.PROD,
                    },
                ],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'test',
                        team: 'test',
                    },
                    {
                        name: 'example-app',
                        team: '',
                    },
                ],
            },
            inputVersion: 10,
            expectedDifference: 0,
        },

        {
            name: 'environment does not exist in the envs',
            inputAppDetails: {
                'example-app': {
                    details: {
                        application: {
                            name: 'example-app',
                            undeploySummary: UndeploySummary.NORMAL,
                            sourceRepoUrl: '',
                            team: '',
                            warnings: [],
                            releases: [
                                {
                                    version: 10,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 12,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                            ],
                        },
                        deployments: {
                            test: {
                                version: 12,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            inputOverview: {
                environmentGroups: [
                    {
                        environmentGroupName: 'test',
                        environments: [
                            {
                                name: 'exampleEnv',
                                locks: {},
                                teamLocks: {},
                                distanceToUpstream: 0,
                                priority: Priority.PROD,
                            },
                        ],
                        distanceToUpstream: 0,
                        priority: Priority.PROD,
                    },
                ],
                gitRevision: '',
                branch: '',
                lightweightApps: [
                    {
                        name: 'test',
                        team: 'test',
                    },
                ],
                manifestRepoUrl: '',
            },
            inputVersion: 10,
            expectedDifference: 0,
        },
        {
            name: 'Simple diff calculation',
            inputAppDetails: {
                [appName]: {
                    details: {
                        application: {
                            name: appName,
                            undeploySummary: UndeploySummary.NORMAL,
                            sourceRepoUrl: '',
                            team: '',
                            warnings: [],
                            releases: [
                                {
                                    version: 10,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 12,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 15,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                            ],
                        },
                        deployments: {
                            [envName]: {
                                version: 10,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            inputOverview: {
                environmentGroups: [
                    {
                        environmentGroupName: 'test',
                        environments: [
                            {
                                name: envName,
                                locks: {},
                                teamLocks: {},
                                distanceToUpstream: 0,
                                priority: Priority.PROD,
                            },
                        ],
                        distanceToUpstream: 0,
                        priority: Priority.PROD,
                    },
                ],
                gitRevision: '',
                branch: '',

                lightweightApps: [
                    {
                        name: 'test',
                        team: 'test',
                    },
                ],
                manifestRepoUrl: '',
            },

            inputVersion: 15,
            expectedDifference: 2,
        },
        {
            name: 'negative diff',
            inputAppDetails: {
                [appName]: {
                    details: {
                        application: {
                            name: appName,
                            undeploySummary: UndeploySummary.NORMAL,
                            sourceRepoUrl: '',
                            team: '',
                            warnings: [],
                            releases: [
                                {
                                    version: 10,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 12,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                            ],
                        },
                        deployments: {
                            [envName]: {
                                version: 12,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            inputOverview: {
                environmentGroups: [
                    {
                        environmentGroupName: 'test',
                        environments: [
                            {
                                name: envName,
                                locks: {},
                                teamLocks: {},
                                distanceToUpstream: 0,
                                priority: Priority.PROD,
                            },
                        ],
                        distanceToUpstream: 0,
                        priority: Priority.PROD,
                    },
                ],
                gitRevision: '',
                branch: '',
                lightweightApps: [
                    {
                        name: 'test',
                        team: 'test',
                    },
                ],
                manifestRepoUrl: '',
            },
            inputVersion: 10,
            expectedDifference: -1,
        },
        {
            name: 'the input version does not exist',
            inputAppDetails: {
                appName: {
                    details: {
                        application: {
                            name: appName,
                            undeploySummary: UndeploySummary.NORMAL,
                            sourceRepoUrl: '',
                            team: '',
                            warnings: [],
                            releases: [
                                {
                                    version: 10,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 12,
                                    sourceCommitId: '',
                                    sourceAuthor: '',
                                    sourceMessage: '',
                                    undeployVersion: false,
                                    prNumber: '',
                                    displayVersion: '',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                            ],
                        },
                        deployments: {
                            [envName]: {
                                version: 12,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            inputOverview: {
                environmentGroups: [
                    {
                        environmentGroupName: 'test',
                        environments: [
                            {
                                name: envName,
                                locks: {},
                                teamLocks: {},
                                distanceToUpstream: 0,
                                priority: Priority.PROD,
                            },
                        ],
                        distanceToUpstream: 0,
                        priority: Priority.PROD,
                    },
                ],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'test',
                        team: 'test',
                    },
                ],
            },
            inputVersion: 11,
            expectedDifference: 0,
        },
    ];
    describe.each(testdata)('with', (testcase) => {
        updateAppDetails.set({});
        it(testcase.name, () => {
            updateActions([]);
            updateAppDetails.set({});
            UpdateOverview.set(testcase.inputOverview);
            updateAppDetails.set(testcase.inputAppDetails);
            const calculatedDiff = renderHook(() => useReleaseDifference(testcase.inputVersion, appName, envName))
                .result.current;
            expect(calculatedDiff).toStrictEqual(testcase.expectedDifference);
        });
    });
});
