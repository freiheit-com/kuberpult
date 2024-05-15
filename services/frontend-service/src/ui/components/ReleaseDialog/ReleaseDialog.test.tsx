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
import { UpdateAction, UpdateOverview, UpdateRolloutStatus, UpdateSidebar } from '../../utils/store';
import { Environment, EnvironmentGroup, Priority, Release, RolloutStatus, UndeploySummary } from '../../../api/api';
import { Spy } from 'spy4js';
import { SideBar } from '../SideBar/SideBar';
import { MemoryRouter } from 'react-router-dom';

const mock_FormattedDate = Spy.mockModule('../FormattedDate/FormattedDate', 'FormattedDate');

describe('Release Dialog', () => {
    const getNode = (overrides: ReleaseDialogProps) => (
        <MemoryRouter>
            <ReleaseDialog {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseDialogProps) => render(getNode(overrides));

    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        rels: Release[];
        envs: Environment[];
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
                version: 2,
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
                },
            ],
            envs: [
                {
                    name: 'prod',
                    locks: {},
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 2,
                            locks: {},
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
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
            props: {
                app: 'test1',
                version: 2,
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
                },
            ],
            envs: [
                {
                    name: 'prod',
                    locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 2,
                            locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
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
            name: 'normal release with deploymentMetadata set',
            props: {
                app: 'test1',
                version: 2,
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
                },
            ],
            envs: [
                {
                    name: 'prod',
                    locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 2,
                            locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: false,
                            deploymentMetaData: { deployAuthor: 'test', deployTime: '1688467491' },
                        },
                    },
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
            name: 'two envs release',
            props: {
                app: 'test1',
                version: 2,
            },
            envs: [
                {
                    name: 'prod',
                    locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 2,
                            locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                            teamLocks: {},
                            team: 'test-team',
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
                {
                    name: 'dev',
                    locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 3,
                            locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                            teamLocks: { teamLock: { message: 'teamLock', lockId: 'ui-teamlock' } },
                            team: 'test-team',
                            queuedVersion: 666,
                            undeployVersion: false,
                        },
                    },
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
                    sourceMessage: 'the other commit message 2',
                    version: 2,
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: 'PR123',
                    sourceAuthor: 'nobody',
                    displayVersion: '2',
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
            data_length: 5,
            teamName: 'test me team',
        },
        {
            name: 'undeploy version release',
            props: {
                app: 'test1',
                version: 4,
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
            applications: {
                [testcase.props.app]: {
                    name: testcase.props.app,
                    releases: testcase.rels,
                    team: testcase.teamName,
                    sourceRepoUrl: 'url',
                    undeploySummary: UndeploySummary.NORMAL,
                    warnings: [],
                },
            },
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: testcase.envs,
                    distanceToUpstream: 2,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        });
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
                    testcase.rels[0].sourceMessage
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
            expect(document.querySelector('.release-env-list')?.children).toHaveLength(testcase.envs.length);
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
                expect(document.querySelector('.env-locks')?.children).toHaveLength(Object.values(env.locks).length);
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
        it('Test using direct call to open function', () => {
            UpdateSidebar.set({ shown: false });
            UpdateSidebar.set({ shown: true });
            expect(UpdateSidebar.get().shown).toBeTruthy();
        });

        describe.each(dataLocks)('click handling', (testcase) => {
            it('Test using deploy button click simulation ' + testcase.name, () => {
                UpdateSidebar.set({ shown: false });
                UpdateAction.set({ actions: [] });
                setTheStore(testcase);

                render(
                    <EnvironmentListItem
                        env={testcase.envs[0]}
                        envGroup={testcase.envGroups[0]}
                        app={testcase.props.app}
                        queuedVersion={0}
                        release={{ ...testcase.rels[0], version: 3 }}
                    />
                );
                const result = querySelectorSafe('.env-card-deploy-btn');
                fireEvent.click(result);
                expect(UpdateSidebar.get().shown).toBeTruthy();
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
                            },
                        },
                    },
                ]);
            });
        });
        it('Test using add lock button click simulation', () => {
            const testcase = data[0];
            UpdateSidebar.set({ shown: false });
            UpdateAction.set({ actions: [] });
            setTheStore(testcase);

            getWrapper(testcase.props);
            render(
                <EnvironmentListItem
                    env={testcase.envs[0]}
                    envGroup={testcase.envGroups[0]}
                    app={testcase.props.app}
                    queuedVersion={0}
                    release={testcase.rels[0]}
                />
            );
            render(<SideBar toggleSidebar={Spy()} />);
            const result = querySelectorSafe('.env-card-add-lock-btn');
            fireEvent.click(result);
            expect(UpdateSidebar.get().shown).toBeTruthy();
            expect(UpdateAction.get().actions).toEqual([
                {
                    action: {
                        $case: 'createEnvironmentApplicationLock',
                        createEnvironmentApplicationLock: {
                            application: 'test1',
                            environment: 'prod',
                            lockId: '',
                            message: '',
                        },
                    },
                },
            ]);
        });
    });
});
