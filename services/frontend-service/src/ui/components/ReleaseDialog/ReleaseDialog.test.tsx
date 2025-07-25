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
import { EnvironmentListItem, ReleaseDialog, ReleaseDialogProps } from './ReleaseDialog';
import { fireEvent, render } from '@testing-library/react';
import {
    AppDetailsResponse,
    AppDetailsState,
    UpdateAction,
    updateAllEnvLocks,
    updateAppDetails,
    UpdateOverview,
    UpdateRolloutStatus,
} from '../../utils/store';
import {
    Environment,
    EnvironmentGroup,
    GetAllEnvTeamLocksResponse,
    Priority,
    Release,
    RolloutStatus,
    UndeploySummary,
} from '../../../api/api';
import { Spy } from 'spy4js';
import { BrowserRouter, MemoryRouter } from 'react-router-dom';

const mock_FormattedDate = Spy.mockModule('../FormattedDate/FormattedDate', 'FormattedDate');
const getNode = (overrides: ReleaseDialogProps) => (
    <MemoryRouter>
        <ReleaseDialog {...overrides} />
    </MemoryRouter>
);
const getWrapper = (overrides: ReleaseDialogProps) => render(getNode(overrides));

describe('Release Dialog', () => {
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        appDetails: { [p: string]: AppDetailsResponse };
        rels: Release[];
        envs: Environment[];
        allEnvLocks: GetAllEnvTeamLocksResponse;
        envGroups: EnvironmentGroup[];
        expect_message: boolean;
        expect_queues: number;
        data_length: number;
        teamName: string;
        rolloutStatus?: {
            application: string;
            environment: string;
            rolloutStatus: RolloutStatus;
            rolloutStatusName: string;
        }[];
    }
    interface dataTLocks {
        name: string;
        props: ReleaseDialogProps;
        rels: Release[];
        envs: Environment[];
        allEnvLocks: GetAllEnvTeamLocksResponse;
        appDetails: { [p: string]: AppDetailsResponse };
        envGroups: EnvironmentGroup[];
        expect_message: boolean;
        expect_queues: number;
        data_length: number;
        teamName: string;
    }
    const dataLocks: dataTLocks[] = [
        {
            name: 'without locks',
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            allEnvLocks: {
                allTeamLocks: {},
                allEnvLocks: {},
            },
            appDetails: {},
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: ['prod'],
                    ciLink: 'givemesomething',
                    revision: 0,
                },
            ],
            envs: [
                {
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'prod',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
            expect_message: true,
            expect_queues: 0,
            data_length: 2,
            teamName: '',
        },
        {
            name: 'with prepublish',
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {},
            allEnvLocks: {
                allTeamLocks: {},
                allEnvLocks: {},
            },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: true,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            envs: [
                {
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'prod',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
            expect_message: true,
            expect_queues: 0,
            data_length: 2,
            teamName: '',
        },
    ];
    const data: dataT[] = [
        {
            name: 'normal release',
            allEnvLocks: {
                allTeamLocks: {},
                allEnvLocks: {
                    prod: {
                        locks: [
                            {
                                message: 'envLock',
                                lockId: 'ui-envlock',
                                ciLink: '',
                                suggestedLifetime: '',
                            },
                        ],
                    },
                },
            },
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    version: 2,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: '#1337',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {
                            production: {
                                locks: [
                                    { message: 'appLock', lockId: 'ui-applock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                        teamLocks: {},
                        deployments: {
                            dev: {
                                version: 1,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    updatedAt: new Date(Date.now()),
                    appDetailState: AppDetailsState.READY,
                    errorMessage: '',
                },
            },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
            ],
            envs: [
                {
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'prod',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
            expect_message: true,
            expect_queues: 0,
            data_length: 3,
            teamName: '',
        },
        {
            name: 'normal release with deploymentMetadata set',
            allEnvLocks: {
                allTeamLocks: {},
                allEnvLocks: {
                    prod: {
                        locks: [{ message: 'envLock', lockId: 'ui-envlock', ciLink: '', suggestedLifetime: '' }],
                    },
                },
            },
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    version: 2,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: '#1337',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {
                            production: {
                                locks: [
                                    { message: 'appLock', lockId: 'ui-applock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                        teamLocks: {},
                        deployments: {
                            dev: {
                                version: 1,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
            ],
            envs: [
                {
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'prod',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
            expect_message: true,
            expect_queues: 0,
            data_length: 3,
            teamName: '',
        },
        {
            name: 'two envs release',
            allEnvLocks: {
                allTeamLocks: {
                    dev: {
                        teamLocks: {
                            test1: {
                                locks: [
                                    { message: 'teamLock', lockId: 'ui-teamlock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                    },
                },
                allEnvLocks: {
                    prod: {
                        locks: [{ message: 'envLock', lockId: 'ui-envlock', ciLink: '', suggestedLifetime: '' }],
                    },
                    dev: {
                        locks: [{ message: 'envLock', lockId: 'ui-envlock', ciLink: '', suggestedLifetime: '' }],
                    },
                },
            },
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    sourceCommitId: 'cafe',
                                    sourceMessage: 'the other commit message 2',
                                    version: 2,
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: 'PR123',
                                    sourceAuthor: 'nobody',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                                {
                                    sourceCommitId: 'cafe',
                                    sourceMessage: 'the other commit message 3',
                                    version: 3,
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: 'PR123',
                                    sourceAuthor: 'nobody',
                                    displayVersion: '3',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {
                            production: {
                                locks: [
                                    { message: 'appLock', lockId: 'ui-applock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                            dev: {
                                locks: [
                                    { message: 'appLock', lockId: 'ui-applock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                        teamLocks: {
                            dev: {
                                locks: [
                                    { message: 'teamLock', lockId: 'ui-teamlock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                        deployments: {
                            prod: {
                                version: 2,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                            dev: {
                                version: 3,
                                queuedVersion: 666,
                                revision: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            envs: [
                {
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
                {
                    name: 'dev',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'prod',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
            rels: [
                {
                    sourceCommitId: 'cafe',
                    sourceMessage: 'the other commit message 3',
                    version: 3,
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: 'PR123',
                    sourceAuthor: 'nobody',
                    displayVersion: '3',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
                {
                    sourceCommitId: 'cafe',
                    sourceMessage: 'the other commit message 2',
                    version: 2,
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: 'PR123',
                    sourceAuthor: 'nobody',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
            ],
            rolloutStatus: [
                {
                    application: 'test1',
                    environment: 'prod',
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_PENDING,
                    rolloutStatusName: 'pending',
                },
                {
                    application: 'test1',
                    environment: 'dev',
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_PROGRESSING,
                    rolloutStatusName: 'progressing',
                },
            ],
            expect_message: true,
            expect_queues: 1,
            data_length: 7,
            teamName: 'test me team',
        },
        {
            name: 'undeploy version release',
            allEnvLocks: {
                allEnvLocks: {},
                allTeamLocks: {},
            },
            props: {
                app: 'test1',
                version: { version: 4, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    version: 4,
                                    sourceAuthor: 'test1',
                                    sourceMessage: '',
                                    sourceCommitId: '',
                                    prNumber: '',
                                    createdAt: new Date(2002),
                                    undeployVersion: true,
                                    displayVersion: '4',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {},
                        teamLocks: {},
                        deployments: {
                            dev: {
                                version: 3,
                                revision: 0,
                                queuedVersion: 666,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            rels: [
                {
                    version: 4,
                    sourceAuthor: 'test1',
                    sourceMessage: '',
                    sourceCommitId: '',
                    prNumber: '',
                    createdAt: new Date(2002),
                    undeployVersion: true,
                    displayVersion: '4',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
            ],
            envs: [],
            envGroups: [],
            expect_message: false,
            expect_queues: 0,
            data_length: 0,
            teamName: '',
        },
    ];

    const setTheStore = (testcase: dataT) => {
        const asMap: { [key: string]: Environment } = {};
        testcase.envs.forEach((obj) => {
            asMap[obj.name] = obj;
        });
        UpdateOverview.set({
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: testcase.envs,
                    distanceToUpstream: 2,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        });
        updateAppDetails.set(testcase.appDetails);
        updateAllEnvLocks.set(testcase.allEnvLocks);
        const status = testcase.rolloutStatus;
        if (status !== undefined) {
            for (const app of status) {
                UpdateRolloutStatus({
                    application: app.application,
                    environment: app.environment,
                    version: 1,
                    rolloutStatus: app.rolloutStatus,
                });
            }
        }
    };

    describe.each(data)(`Renders a Release Dialog`, (testcase) => {
        it(testcase.name, () => {
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            if (testcase.expect_message) {
                expect(document.querySelector('.release-dialog-message')?.textContent).toContain(
                    testcase.rels[testcase.rels.length - 1].sourceMessage
                );
            } else {
                expect(document.querySelector('.release-dialog-message') === undefined);
            }
            expect(document.querySelectorAll('.env-card-data')).toHaveLength(testcase.data_length);
            expect(document.querySelectorAll('.env-card-data-queue')).toHaveLength(testcase.expect_queues);
        });
    });

    describe.each(data)(`Renders the environment cards`, (testcase) => {
        it(testcase.name, () => {
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            expect(document.querySelector('.release-dialog-environment-group-lane__body')?.children).toHaveLength(
                testcase.envs.length
            );
        });
    });

    describe.each(data)(`Renders the environment locks`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            expect(document.body).toMatchSnapshot();
            expect(document.querySelectorAll('.release-env-group-list')).toHaveLength(1);
            testcase.envs.forEach((env) => {
                const envLocks = testcase.allEnvLocks.allEnvLocks[env.name]?.locks ?? [];
                expect(document.querySelector('.env-locks')?.children).toHaveLength(envLocks.length);
            });
        });
    });

    describe.each(data)(`Renders the queuedVersion`, (testcase) => {
        it(testcase.name, () => {
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            expect(document.querySelectorAll('.env-card-data-queue')).toHaveLength(testcase.expect_queues);
        });
    });

    describe.each(data)(`Renders the rollout status`, (testcase) => {
        const status = testcase.rolloutStatus;
        if (status === undefined) {
            return;
        }
        it(testcase.name, () => {
            const statusCount: { [status: string]: number } = {};
            for (const app of status) {
                statusCount[app.rolloutStatusName] = (statusCount[app.rolloutStatusName] ?? 0) + 1;
            }
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            for (const [descr, count] of Object.entries(statusCount)) {
                expect(document.querySelectorAll('.rollout__description_' + descr)).toHaveLength(count);
            }
        });
    });

    const querySelectorSafe = (selectors: string): Element => {
        const result = document.querySelector(selectors);
        if (!result) {
            throw new Error('did not find in selector in document ' + selectors);
        }
        return result;
    };

    describe(`Test automatic cart opening`, () => {
        describe.each(dataLocks)('click handling', (testcase) => {
            it('Test using deploy button click simulation ' + testcase.name, () => {
                UpdateAction.set({ actions: [] });
                setTheStore(testcase);

                render(
                    <BrowserRouter>
                        <EnvironmentListItem
                            env={testcase.envs[0]}
                            envGroup={testcase.envGroups[0]}
                            app={testcase.props.app}
                            queuedVersion={0}
                            release={{ ...testcase.rels[0], version: 3 }}
                        />
                    </BrowserRouter>
                );
                const result = querySelectorSafe('.env-card-deploy-btn');
                if (testcase.rels[0].isPrepublish) {
                    expect(result).toBeDisabled();
                } else {
                    fireEvent.click(result);
                    expect(UpdateAction.get().actions).toEqual([
                        {
                            action: {
                                $case: 'deploy',
                                deploy: {
                                    application: 'test1',
                                    environment: 'prod',
                                    ignoreAllLocks: false,
                                    lockBehavior: 2,
                                    version: 3,
                                    revision: 0,
                                },
                            },
                        },
                        {
                            action: {
                                $case: 'createEnvironmentApplicationLock',
                                createEnvironmentApplicationLock: {
                                    application: 'test1',
                                    environment: 'prod',
                                    lockId: '',
                                    message: '',
                                    ciLink: '',
                                },
                            },
                        },
                    ]);
                }
            });
        });
    });
});

describe('Release Dialog CI Links', () => {
    const getNode = (overrides: ReleaseDialogProps) => (
        <MemoryRouter>
            <ReleaseDialog {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseDialogProps) => render(getNode(overrides));
    interface ReleaseDataT {
        appName: string;
        version: number;
        ciLink: string;
    }
    interface DeploymentDataT {
        appName: string;
        version: number;
        envName: string;
        ciLink: string;
    }
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        appDetails: { [p: string]: AppDetailsResponse };
        deploymentData: DeploymentDataT[];
        releaseData: ReleaseDataT;
        envs: Environment[];
        envGroups: EnvironmentGroup[];
    }

    const ciLinksData: dataT[] = [
        {
            name: 'normal release',
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    version: 2,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: '#1337',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {
                            production: {
                                locks: [
                                    { message: 'appLock', lockId: 'ui-applock', ciLink: '', suggestedLifetime: '' },
                                ],
                            },
                        },
                        teamLocks: {},
                        deployments: {
                            dev: {
                                version: 1,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    updatedAt: new Date(Date.now()),
                    appDetailState: AppDetailsState.READY,
                    errorMessage: '',
                },
            },
            deploymentData: [
                {
                    version: 1,
                    envName: 'dev',
                    appName: 'test1',
                    ciLink: 'www.somewebsite.com',
                },
            ],
            releaseData: {
                version: 1,
                appName: 'test1',
                ciLink: 'www.somewebsite.com',
            },
            envs: [
                {
                    name: 'dev',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            envGroups: [
                {
                    // this data should never appear (group with no envs with a well-defined priority), but we'll make it for the sake of the test.
                    distanceToUpstream: 0,
                    environmentGroupName: 'dev',
                    environments: [],
                    priority: Priority.UPSTREAM,
                },
            ],
        },
    ];

    const setTheStore = (testcase: dataT) => {
        const asMap: { [key: string]: Environment } = {};
        testcase.envs.forEach((obj) => {
            asMap[obj.name] = obj;
        });
        UpdateOverview.set({
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: testcase.envs,
                    distanceToUpstream: 2,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        });
        updateAppDetails.set(testcase.appDetails);
    };

    describe.each(ciLinksData)(`Renders ci links for release and deployments`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            if (testcase.releaseData.ciLink !== '') {
                expect(document.getElementById('ciLink')?.getAttribute('href')).toContain(testcase.releaseData.ciLink);
            }

            testcase.deploymentData.forEach((curr) =>
                expect(
                    document
                        .getElementById('deployment-ci-link-' + curr.envName + '-' + curr.appName)
                        ?.getAttribute('href')
                ).toContain(curr.ciLink)
            );
        });
    });
});

describe('Rollout Status for AA environments', () => {
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        appDetails: { [p: string]: AppDetailsResponse };
        rels: Release[];
        envs: Environment[];
        teamName: string;
        rolloutStatus: {
            application: string;
            environment: string;
            rolloutStatus: RolloutStatus;
            rolloutStatusName: string;
        }[];
        expectedStatusIcon: RolloutStatus;
        expectedRolloutDetails: { [name: string]: RolloutStatus };
    }

    const data: dataT[] = [
        {
            name: 'normal rollout status',
            props: {
                app: 'test1',
                version: { version: 2, revision: 0 },
            },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    sourceCommitId: 'cafe',
                                    sourceMessage: 'the other commit message 2',
                                    version: 2,
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: 'PR123',
                                    sourceAuthor: 'nobody',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                                {
                                    sourceCommitId: 'cafe',
                                    sourceMessage: 'the other commit message 3',
                                    version: 3,
                                    createdAt: new Date(2002),
                                    undeployVersion: false,
                                    prNumber: 'PR123',
                                    sourceAuthor: 'nobody',
                                    displayVersion: '3',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: 'www.somewebsite.com',
                                    revision: 0,
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {},
                        teamLocks: {},
                        deployments: {
                            prod: {
                                version: 2,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                            dev: {
                                version: 3,
                                revision: 0,
                                queuedVersion: 666,
                                undeployVersion: false,
                                deploymentMetaData: {
                                    ciLink: 'www.somewebsite.com',
                                    deployAuthor: 'somebody',
                                    deployTime: 'sometime',
                                },
                            },
                        },
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            envs: [
                {
                    config: {
                        argoConfigs: {
                            configs: [
                                {
                                    concreteEnvName: 'test-1',
                                    syncWindows: [],
                                    accessList: [],
                                    applicationAnnotations: {},
                                    ignoreDifferences: [],
                                    syncOptions: [],
                                },
                                {
                                    concreteEnvName: 'test-2',
                                    syncWindows: [],
                                    accessList: [],
                                    applicationAnnotations: {},
                                    ignoreDifferences: [],
                                    syncOptions: [],
                                },
                            ],
                            commonEnvPrefix: 'aa',
                        },
                    },
                    name: 'prod',
                    distanceToUpstream: 0,
                    priority: Priority.OTHER,
                },
                {
                    name: 'dev',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],

            rels: [
                {
                    sourceCommitId: 'cafe',
                    sourceMessage: 'the other commit message 3',
                    version: 3,
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: 'PR123',
                    sourceAuthor: 'nobody',
                    displayVersion: '3',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
                {
                    sourceCommitId: 'cafe',
                    sourceMessage: 'the other commit message 2',
                    version: 2,
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: 'PR123',
                    sourceAuthor: 'nobody',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: 'www.somewebsite.com',
                    revision: 0,
                },
            ],
            rolloutStatus: [
                {
                    application: 'test1',
                    environment: 'aa-prod-test-1',
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_ERROR,
                    rolloutStatusName: 'error',
                },
                {
                    application: 'test1',
                    environment: 'aa-prod-test-2',
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_PENDING,
                    rolloutStatusName: 'pending',
                },
                {
                    application: 'test1',
                    environment: 'dev',
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_PROGRESSING,
                    rolloutStatusName: 'progressing',
                },
            ],
            expectedStatusIcon: RolloutStatus.ROLLOUT_STATUS_ERROR,
            expectedRolloutDetails: {
                prod: RolloutStatus.ROLLOUT_STATUS_ERROR,
                dev: RolloutStatus.ROLLOUT_STATUS_PROGRESSING,
            },
            teamName: 'test me team',
        },
    ];

    const setTheStore = (testcase: dataT) => {
        UpdateOverview.set({
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: testcase.envs,
                    distanceToUpstream: 2,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        });
        updateAppDetails.set(testcase.appDetails);
        const status = testcase.rolloutStatus;
        if (status !== undefined) {
            for (const app of status) {
                UpdateRolloutStatus({
                    application: app.application,
                    environment: app.environment,
                    version: 1,
                    rolloutStatus: app.rolloutStatus,
                });
            }
        }
    };

    describe.each(data)(`Rollout Status`, (testcase) => {
        it(testcase.name, async () => {
            // when
            setTheStore(testcase);
            getWrapper(testcase.props);
            for (const [envName, status] of Object.entries(testcase.expectedRolloutDetails)) {
                expect(
                    document
                        .getElementById(envName)
                        ?.getElementsByClassName('rollout__description_' + rolloutStatusName(status))[0] //each should only have 1
                ).toHaveTextContent(rolloutStatusTextContent(status));
            }
        });
    });
});

const rolloutStatusTextContent = (status: RolloutStatus): string => {
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            return '✓ Done';
        case RolloutStatus.ROLLOUT_STATUS_PROGRESSING:
            return '↻ In progress';
        case RolloutStatus.ROLLOUT_STATUS_PENDING:
            return '⧖ Pending';
        case RolloutStatus.ROLLOUT_STATUS_ERROR:
            return '! Failed';
        case RolloutStatus.ROLLOUT_STATUS_UNHEALTHY:
            return '⚠ Unhealthy';
        default:
            return '? Unknown';
    }
};

const rolloutStatusName = (status: RolloutStatus): string => {
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            return 'successful';
        case RolloutStatus.ROLLOUT_STATUS_PROGRESSING:
            return 'progressing';
        case RolloutStatus.ROLLOUT_STATUS_PENDING:
            return 'pending';
        case RolloutStatus.ROLLOUT_STATUS_ERROR:
            return 'error';
        case RolloutStatus.ROLLOUT_STATUS_UNHEALTHY:
            return 'unhealthy';
        default:
            return 'unknown';
    }
};
