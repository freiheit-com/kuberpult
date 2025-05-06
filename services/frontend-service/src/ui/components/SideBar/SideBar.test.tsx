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
import { TopAppBar } from '../TopAppBar/TopAppBar';
import { MemoryRouter } from 'react-router-dom';
import { BatchAction, LockBehavior, ReleaseTrainRequest_TargetType, UndeploySummary } from '../../../api/api';
import {
    addAction,
    AppDetailsResponse,
    AppDetailsState,
    appendAction,
    deleteAction,
    deleteAllActions,
    DisplayLock,
    SnackbarStatus,
    updateActions,
    updateAppDetails,
    UpdateSnackbar,
    useActions,
} from '../../utils/store';
import { ActionDetails, ActionTypes, getActionDetails, showFailedActionMessage, SideBar } from './SideBar';

describe('Sidebar shows list of actions', () => {
    interface dataT {
        name: string;
        actions: BatchAction[];
        expectedNumOfActions: number;
    }

    const data: dataT[] = [
        {
            name: '2 results',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedNumOfActions: 2,
        },
        {
            name: '1 results, repeated',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
            ],
            expectedNumOfActions: 1,
        },
        {
            name: '3 results',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedNumOfActions: 3,
        },
        {
            name: '2 results, repeated',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedNumOfActions: 2,
        },
        {
            name: '0 results',
            actions: [],
            expectedNumOfActions: 0,
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <MemoryRouter>
                <TopAppBar {...defaultProps} {...overrides} />{' '}
            </MemoryRouter>
        );
    };
    const getWrapper = (overrides?: {}) => render(getNode(overrides));

    describe.each(data)('', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions(testcase.actions);
            // when
            const { container } = getWrapper({});
            // then
            expect(container.getElementsByClassName('mdc-drawer-sidebar-list')[0].children).toHaveLength(
                testcase.expectedNumOfActions
            );
        });
    });
});

describe('Sidebar test deletebutton', () => {
    interface dataT {
        name: string;
        actions: BatchAction[];
        expectedNumOfActions: number;
    }

    const data: dataT[] = [
        {
            name: '2 results',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedNumOfActions: 1,
        },
        {
            name: '3 results',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedNumOfActions: 2,
        },
        {
            name: '0 results',
            actions: [],
            expectedNumOfActions: 0,
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <MemoryRouter>
                <TopAppBar {...defaultProps} {...overrides} />{' '}
            </MemoryRouter>
        );
    };
    const getWrapper = (overrides?: {}) => render(getNode(overrides));

    describe.each(data)('', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions(testcase.actions);
            // when
            const { container } = getWrapper({});
            const svg = container.getElementsByClassName('mdc-drawer-sidebar-list-item-delete-icon')[0];
            if (svg) {
                const button = svg.parentElement;
                if (button) button.click();
            }
            // then
            expect(container.getElementsByClassName('mdc-drawer-sidebar-list')[0].children).toHaveLength(
                testcase.expectedNumOfActions
            );
        });
    });
});

describe('Action Store functionality', () => {
    interface dataT {
        name: string;
        actions: BatchAction[];
        deleteActions?: BatchAction[];
        expectedActions: BatchAction[];
    }

    const dataGetSet: dataT[] = [
        {
            name: '1 action',
            actions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
        },
        {
            name: 'Empty action store',
            actions: [],
            expectedActions: [],
        },
        {
            name: '2 different type of actions',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
        },
        {
            name: '2 actions of the same type',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
        },
    ];

    describe.each(dataGetSet)('Test getting actions from the store and setting the store from an array', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions(testcase.actions);
            // when
            const actions = renderHook(() => useActions()).result.current;
            // then
            expect(actions).toStrictEqual(testcase.expectedActions);
        });
    });

    const dataAdding: dataT[] = [
        {
            name: '1 action',
            actions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
        },
        {
            name: 'Empty action store',
            actions: [],
            expectedActions: [],
        },
        {
            name: '2 different type of actions',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
        },
        {
            name: '2 actions of the same type',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
        },
    ];

    describe.each(dataAdding)('Test adding actions to the store', (testcase) => {
        it(testcase.name, () => {
            // given
            deleteAllActions();
            testcase.actions.forEach((action) => {
                addAction(action);
            });
            // when
            const actions = renderHook(() => useActions()).result.current;
            // then
            expect(actions).toStrictEqual(testcase.expectedActions);
        });
    });

    const dataDeleting: dataT[] = [
        {
            name: 'delete 1 action - 0 remain',
            actions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            deleteActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [],
        },
        {
            name: 'delete 1 action (different action type, same app) - 1 remains',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            deleteActions: [{ action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
        },
        {
            name: 'delete 1 action (same action type, different app) - 1 remains',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            deleteActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } }],
        },
        {
            name: 'delete 1 action from empty array - 0 remain',
            actions: [],
            deleteActions: [{ action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } }],
            expectedActions: [],
        },
        {
            name: 'delete 2 actions - 1 remain',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            deleteActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } }],
        },
    ];

    describe.each(dataDeleting)('Test deleting actions', (testcase) => {
        it(testcase.name, () => {
            // given
            updateActions(testcase.actions);
            // when
            testcase.deleteActions?.map((action) => deleteAction(action));
            const actions = renderHook(() => useActions()).result.current;
            // then
            expect(actions).toStrictEqual(testcase.expectedActions);
        });
    });
});

describe('Action details', () => {
    interface dataT {
        name: string;
        action: BatchAction;
        envLocks?: DisplayLock[];
        appLocks?: DisplayLock[];
        teamLocks?: DisplayLock[];
        expectedDetails: ActionDetails;
    }
    const data: dataT[] = [
        {
            name: 'test createEnvironmentLock action',
            action: {
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: {
                        environment: 'foo',
                        lockId: 'ui-v2-1337',
                        message: 'bar',
                        ciLink: '',
                        suggestedLifeTime: '2h',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.CreateEnvironmentLock,
                name: 'Create Env Lock',
                dialogTitle: 'Are you sure you want to add this environment lock?',
                tooltip:
                    'An environment lock will prevent automated process from changing the deployed version - note that kuberpult users can still deploy despite locks.',
                summary: 'Create new environment lock on foo',
                environment: 'foo',
            },
        },
        {
            name: 'test deleteEnvironmentLock action',
            action: {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: { environment: 'foo', lockId: 'ui-v2-1337' },
                },
            },
            envLocks: [
                {
                    lockId: 'ui-v2-1337',
                    environment: 'foo',
                    message: 'bar',
                    ciLink: '',
                    suggestedLifetime: '',
                },
            ],
            expectedDetails: {
                type: ActionTypes.DeleteEnvironmentLock,
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                summary: 'Delete environment lock on foo with the message: "bar"',
                tooltip: 'This will only remove the lock, it will not automatically deploy anything.',
                environment: 'foo',
                lockId: 'ui-v2-1337',
                lockMessage: 'bar',
            },
        },
        {
            name: 'test createEnvironmentApplicationLock action',
            action: {
                action: {
                    $case: 'createEnvironmentApplicationLock',
                    createEnvironmentApplicationLock: {
                        environment: 'foo',
                        application: 'bread',
                        lockId: 'ui-v2-1337',
                        message: 'bar',
                        ciLink: '',
                        suggestedLifeTime: '',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.CreateApplicationLock,
                name: 'Create App Lock',
                dialogTitle: 'Are you sure you want to add this application lock?',
                summary: 'Create new application lock for "bread" on foo',
                tooltip:
                    'An app lock will prevent automated process from changing the deployed version - note that kuberpult users can still deploy despite locks.',
                environment: 'foo',
                application: 'bread',
            },
        },
        {
            name: 'test deleteEnvironmentApplicationLock action',
            action: {
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        environment: 'foo',
                        application: 'bar',
                        lockId: 'ui-v2-1337',
                    },
                },
            },
            appLocks: [
                {
                    lockId: 'ui-v2-1337',
                    environment: 'foo',
                    message: 'bar',
                    application: 'bar',
                    ciLink: '',
                    suggestedLifetime: '',
                },
            ],
            expectedDetails: {
                type: ActionTypes.DeleteApplicationLock,
                name: 'Delete App Lock',
                dialogTitle: 'Are you sure you want to delete this application lock?',
                summary: 'Delete application lock for "bar" on foo with the message: "bar"',
                tooltip: 'This will only remove the lock, it will not automatically deploy anything.',
                environment: 'foo',
                application: 'bar',
                lockId: 'ui-v2-1337',
                lockMessage: 'bar',
            },
        },
        {
            name: 'test createEnvironmentTeamLock action',
            action: {
                action: {
                    $case: 'createEnvironmentTeamLock',
                    createEnvironmentTeamLock: {
                        environment: 'foo',
                        team: 'sre-team',
                        lockId: 'ui-v2-1339',
                        message: 'bar',
                        ciLink: '',
                        suggestedLifeTime: '1w',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.CreateEnvironmentTeamLock,
                name: 'Create Team Lock',
                dialogTitle: 'Are you sure you want to add this team lock?',
                summary: 'Create new team lock for "sre-team" on foo',
                tooltip:
                    'A team lock will prevent automated process from changing the deployed version - note that kuberpult users can still deploy despite locks.',
                environment: 'foo',
                team: 'sre-team',
            },
        },
        {
            name: 'test deleteEnvironmentTeamLock action',
            action: {
                action: {
                    $case: 'deleteEnvironmentTeamLock',
                    deleteEnvironmentTeamLock: { environment: 'foo', team: 'bar', lockId: 'ui-v2-1338' },
                },
            },
            teamLocks: [
                {
                    lockId: 'ui-v2-1338',
                    environment: 'foo',
                    message: 'bar',
                    team: 'bar',
                    ciLink: '',
                    suggestedLifetime: '',
                },
            ],
            expectedDetails: {
                type: ActionTypes.DeleteEnvironmentTeamLock,
                name: 'Delete Team Lock',
                dialogTitle: 'Are you sure you want to delete this team lock?',
                summary: 'Delete team lock for "bar" on foo with the message: "bar"',
                tooltip: 'This will only remove the lock, it will not automatically deploy anything.',
                environment: 'foo',
                team: 'bar',
                lockId: 'ui-v2-1338',
                lockMessage: 'bar',
            },
        },
        {
            name: 'test deploy action',
            action: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'foo',
                        application: 'bread',
                        version: 1337,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Please be aware:',
                summary: 'Deploy version 1337 of "bread" to foo',
                tooltip: '',
                environment: 'foo',
                application: 'bread',
                version: 1337,
            },
        },
        {
            name: 'test deploy action - forward',
            action: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'foo',
                        application: 'bread',
                        version: 1338,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Please be aware:',
                summary: 'Advancing by 1 releases up to version 1338 of bread to foo',
                tooltip: '',
                environment: 'foo',
                application: 'bread',
                version: 1338,
            },
        },
        {
            name: 'test deploy action - rollback',
            action: {
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: 'foo',
                        application: 'bread',
                        version: 1336,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.IGNORE,
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Please be aware:',
                summary: 'Rolling back by 1 releases down to version 1336 of bread to foo',
                tooltip: '',
                environment: 'foo',
                application: 'bread',
                version: 1336,
            },
        },
        {
            name: 'test prepareUndeploy action',
            action: {
                action: {
                    $case: 'prepareUndeploy',
                    prepareUndeploy: {
                        application: 'foo',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.PrepareUndeploy,
                name: 'Prepare Undeploy',
                dialogTitle: 'Are you sure you want to start undeploy?',
                tooltip:
                    'The new version will go through the same cycle as any other versions' +
                    ' (e.g. development->staging->production). ' +
                    'The behavior is similar to any other version that is created normally.',
                summary: 'Prepare undeploy version for Application "foo"',
                application: 'foo',
            },
        },
        {
            name: 'test undeploy action',
            action: {
                action: {
                    $case: 'undeploy',
                    undeploy: {
                        application: 'foo',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.Undeploy,
                name: 'Undeploy',
                dialogTitle: 'Are you sure you want to undeploy this application?',
                tooltip: 'This application will be deleted permanently',
                summary: 'Undeploy and delete Application "foo"',
                application: 'foo',
            },
        },
        {
            name: 'test delete env from app action',
            action: {
                action: {
                    $case: 'deleteEnvFromApp',
                    deleteEnvFromApp: {
                        environment: 'dev',
                        application: 'foo',
                    },
                },
            },
            expectedDetails: {
                type: ActionTypes.DeleteEnvFromApp,
                name: 'Delete an Environment from App',
                dialogTitle: 'Are you sure you want to delete environments from this application?',
                tooltip: 'These environments will be deleted permanently from this application',
                summary: 'Delete environment "dev" from application "foo"',
                application: 'foo',
            },
        },
        {
            name: 'test releaseTrain action',
            action: {
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
            expectedDetails: {
                type: ActionTypes.ReleaseTrain,
                name: 'Release Train',
                dialogTitle: 'Are you sure you want to run a Release Train',
                summary: 'Run release train to environment dev',
                tooltip: '',
                environment: 'dev',
            },
        },
    ];

    describe.each(data)('Test getActionDetails function', (testcase) => {
        it(testcase.name, () => {
            const envLocks = testcase.envLocks || [];
            const appLocks = testcase.appLocks || [];
            const teamLocks = testcase.teamLocks || [];
            const appDetails: { [key: string]: AppDetailsResponse } = {
                bread: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    version: 1338,
                                    sourceAuthor: 'SomeAuthor',
                                    sourceMessage: 'some message',
                                    sourceCommitId: 'somecommitid',
                                    undeployVersion: true,
                                    prNumber: '',
                                    displayVersion: '1337',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: ['foo'],
                                    ciLink: '',
                                },
                                {
                                    version: 1337,
                                    sourceAuthor: 'SomeAuthor',
                                    sourceMessage: 'some message',
                                    sourceCommitId: 'somecommitid',
                                    undeployVersion: true,
                                    prNumber: '',
                                    displayVersion: '1337',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: ['foo'],
                                    ciLink: '',
                                },
                                {
                                    version: 1336,
                                    sourceAuthor: 'SomeAuthor',
                                    sourceMessage: 'some message',
                                    sourceCommitId: 'somecommitid',
                                    undeployVersion: true,
                                    prNumber: '',
                                    displayVersion: '1337',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: ['foo'],
                                    ciLink: '',
                                },
                            ],
                            sourceRepoUrl: 'http://foo.com',
                            team: 'dummy',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {},
                        teamLocks: {},
                        deployments: {
                            foo: {
                                version: 1337,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            };
            updateAppDetails.set(appDetails);
            const obtainedDetails = renderHook(() =>
                getActionDetails(testcase.action, appLocks, envLocks, teamLocks, appDetails)
            ).result.current;
            expect(obtainedDetails).toStrictEqual(testcase.expectedDetails);
        });
    });

    describe('Sidebar shows the number of planned actions', () => {
        interface dataT {
            name: string;
            actions: BatchAction[];
            expectedTitle: string;
        }

        const data: dataT[] = [
            {
                name: '2 results',
                actions: [
                    { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                    { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
                ],
                expectedTitle: 'Planned Actions (2)',
            },
            {
                name: '1 results, repeated',
                actions: [
                    { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                    { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                ],
                expectedTitle: 'Planned Actions (1)',
            },
            {
                name: '0 results',
                actions: [],
                expectedTitle: 'Planned Actions',
            },
        ];

        const getNode = (overrides?: {}): JSX.Element | any => {
            // given
            const defaultProps: any = {
                children: null,
            };
            return (
                <MemoryRouter>
                    <SideBar {...defaultProps} {...overrides} />
                </MemoryRouter>
            );
        };
        const getWrapper = (overrides?: {}) => render(getNode(overrides));

        describe.each(data)('', (testcase) => {
            it(testcase.name, () => {
                updateActions(testcase.actions);
                const { container } = getWrapper({});
                expect(container.getElementsByClassName('sub-headline1')[0].textContent).toBe(testcase.expectedTitle);
            });
        });
    });
    describe('Sidebar shows updates number of planned actions', () => {
        interface dataT {
            name: string;
            actions: BatchAction[];
            expectedTitle: string;
            appDetails: { [key: string]: AppDetailsResponse };
        }

        const data: dataT[] = [
            {
                name: 'add 2 actions',
                actions: [
                    { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                    { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
                ],
                expectedTitle: 'Planned Actions (3)',
                appDetails: {
                    nmww: {
                        details: {
                            application: {
                                name: 'nmww',
                                releases: [
                                    {
                                        version: 1337,
                                        sourceAuthor: 'SomeAuthor',
                                        sourceMessage: 'some message',
                                        sourceCommitId: 'somecommitid',
                                        undeployVersion: true,
                                        prNumber: '',
                                        displayVersion: '1337',
                                        isMinor: false,
                                        isPrepublish: false,
                                        environments: ['foo'],
                                        ciLink: '',
                                    },
                                ],
                                sourceRepoUrl: 'http://foo.com',
                                team: 'dummy',
                                undeploySummary: UndeploySummary.NORMAL,
                                warnings: [],
                            },
                            appLocks: {},
                            teamLocks: {},
                            deployments: {},
                        },
                        appDetailState: AppDetailsState.READY,
                        updatedAt: new Date(Date.now()),
                        errorMessage: '',
                    },
                },
            },
            {
                name: 'Add another action',
                actions: [
                    {
                        action: {
                            $case: 'deploy',
                            deploy: {
                                environment: 'foo',
                                application: 'bread',
                                version: 1337,
                                ignoreAllLocks: false,
                                lockBehavior: LockBehavior.IGNORE,
                            },
                        },
                    },
                ],
                expectedTitle: 'Planned Actions (4)',
                appDetails: {
                    bread: {
                        details: {
                            application: {
                                name: 'bread',
                                releases: [
                                    {
                                        version: 1337,
                                        sourceAuthor: 'SomeAuthor',
                                        sourceMessage: 'some message',
                                        sourceCommitId: 'somecommitid',
                                        undeployVersion: false,
                                        prNumber: '',
                                        displayVersion: '1337',
                                        isMinor: false,
                                        isPrepublish: false,
                                        environments: ['foo'],
                                        ciLink: '',
                                    },
                                ],
                                sourceRepoUrl: 'http://foo.com',
                                team: 'dummy',
                                undeploySummary: UndeploySummary.NORMAL,
                                warnings: [],
                            },
                            appLocks: {},
                            teamLocks: {},
                            deployments: {},
                        },
                        appDetailState: AppDetailsState.READY,
                        updatedAt: new Date(Date.now()),
                        errorMessage: '',
                    },
                },
            },
            {
                name: 'Add 2 more actions actions',
                actions: [
                    { action: { $case: 'undeploy', undeploy: { application: 'test2' } } },
                    { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'test2' } } },
                ],
                expectedTitle: 'Planned Actions (6)',
                appDetails: {
                    test2: {
                        details: {
                            application: {
                                name: 'test2',
                                releases: [
                                    {
                                        version: 1337,
                                        sourceAuthor: 'SomeAuthor',
                                        sourceMessage: 'some message',
                                        sourceCommitId: 'somecommitid',
                                        undeployVersion: true,
                                        prNumber: '',
                                        displayVersion: '1337',
                                        isMinor: false,
                                        isPrepublish: false,
                                        environments: ['foo'],
                                        ciLink: '',
                                    },
                                ],
                                sourceRepoUrl: 'http://foo.com',
                                team: 'dummy',
                                undeploySummary: UndeploySummary.NORMAL,
                                warnings: [],
                            },
                            appLocks: {},
                            teamLocks: {},
                            deployments: {},
                        },
                        appDetailState: AppDetailsState.READY,
                        updatedAt: new Date(Date.now()),
                        errorMessage: '',
                    },
                },
            },
        ];

        const getNode = (overrides?: {}): JSX.Element | any => {
            // given
            const defaultProps: any = {
                children: null,
            };
            return (
                <MemoryRouter>
                    <SideBar {...defaultProps} {...overrides} />
                </MemoryRouter>
            );
        };

        const getWrapper = (overrides?: {}) => render(getNode(overrides));
        it('Create an action initially', () => {
            updateActions([{ action: { $case: 'undeploy', undeploy: { application: 'test' } } }]);
            const { container } = getWrapper({});
            expect(container.getElementsByClassName('sub-headline1')[0].textContent).toBe('Planned Actions (1)');
        });
        describe.each(data)('', (testcase) => {
            updateAppDetails.set(testcase.appDetails);
            it(testcase.name, () => {
                appendAction(testcase.actions);
                const { container } = getWrapper({});
                expect(container.getElementsByClassName('sub-headline1')[0].textContent).toBe(testcase.expectedTitle);
            });
        });
        describe('Deleting an action from the cart', () => {
            it('Test deleting an an action', () => {
                updateActions([{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }]);
                appendAction([{ action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } }]);
                // Here we expect the value to be Planned Actions (1)
                const expected = 'Planned Actions (1)';
                const { container } = getWrapper({});
                const svg = container.getElementsByClassName('mdc-drawer-sidebar-list-item-delete-icon')[0];
                if (svg) {
                    const button = svg.parentElement;
                    if (button) button.click();
                }
                expect(container.getElementsByClassName('sub-headline1')[0].textContent).toBe(expected);
            });
        });
    });
});

describe('Action failed after applying ', () => {
    interface TestError {
        code: number;
        message: string;
        reason?: string;
    }

    const fakeTransformerBatchError = (index: number, reason: string): TestError => ({
        code: 1,
        reason,
        message: `error at index ${index} of transformer batch: ${reason}`,
    });

    interface dataT {
        name: string;
        actions: BatchAction[];
        envLocks?: DisplayLock[];
        appLocks?: DisplayLock[];
        teamLocks?: DisplayLock[];
        error: TestError;
        expectedMessage: string;
    }

    const data: dataT[] = [
        {
            name: 'test one action',
            actions: [
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: { environment: 'foo', lockId: 'ui-v2-1337', message: 'bar', ciLink: '' },
                    },
                },
            ],
            error: fakeTransformerBatchError(0, 'it failed'),
            expectedMessage: 'Create new environment lock on foo failed: it failed. Please try again',
        },
        {
            name: 'test multiple actions',
            actions: [
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: { environment: 'foo', lockId: 'ui-v2-1337', message: 'bar', ciLink: '' },
                    },
                },
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: {
                            environment: 'bar',
                            lockId: 'ui-v1-1337',
                            message: 'foo',
                            ciLink: '',
                        },
                    },
                },
            ],
            error: fakeTransformerBatchError(1, 'it failed'),
            expectedMessage: 'Create new environment lock on bar failed: it failed. Please try again',
        },
        {
            name: 'test permission denied error',
            actions: [
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: { environment: 'foo', lockId: 'ui-v2-1337', message: 'bar', ciLink: '' },
                    },
                },
            ],
            error: { code: 7, message: 'permission denied' },
            expectedMessage: 'permission denied',
        },
        {
            name: 'test error message not matching regex',
            actions: [
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: { environment: 'foo', lockId: 'ui-v2-1337', message: 'bar', ciLink: '' },
                    },
                },
            ],
            error: { code: 1, message: 'something' },
            expectedMessage: 'Actions were not applied. Please try again',
        },
    ];

    describe.each(data)('', (testcase) => {
        it(testcase.name, () => {
            const envLocks = testcase.envLocks || [];
            const appLocks = testcase.appLocks || [];
            const teamLocks = testcase.teamLocks || [];
            const appDetails: { [key: string]: AppDetailsResponse } = {};
            showFailedActionMessage(testcase.error, testcase.actions, appLocks, envLocks, teamLocks, appDetails);

            expect(UpdateSnackbar.get().show).toBe(true);
            expect(UpdateSnackbar.get().status).toBe(SnackbarStatus.ERROR);
            expect(UpdateSnackbar.get().content).toBe(testcase.expectedMessage);
        });
    });
});
