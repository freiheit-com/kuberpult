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
import { ReleaseCard, ReleaseCardProps } from './ReleaseCard';
import { render } from '@testing-library/react';
import {
    AppDetailsResponse,
    AppDetailsState,
    ReleaseNumbers,
    updateAppDetails,
    UpdateGitSyncStatus,
    UpdateOverview,
    UpdateRolloutStatus,
} from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';
import {
    Environment,
    EnvironmentGroup,
    GetGitSyncStatusResponse,
    Priority,
    Release,
    RolloutStatus,
    StreamStatusResponse,
    GitSyncStatus,
    UndeploySummary,
} from '../../../api/api';
import { Spy } from 'spy4js';

const mock_FormattedDate = Spy.mockModule('../FormattedDate/FormattedDate', 'FormattedDate');

describe('Release Card', () => {
    const getNode = (overrides: ReleaseCardProps) => (
        <MemoryRouter>
            <ReleaseCard {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseCardProps) => render(getNode(overrides));

    type TestData = {
        name: string;
        props: {
            app: string;
            versionInfo: ReleaseNumbers;
        };
        rels: Release[];
        environments: { [key: string]: Environment };
        appDetails: { [key: string]: AppDetailsResponse };
    };
    const data: TestData[] = [
        {
            name: 'using a sample release - useRelease hook',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test1: {
                    details: {
                        application: {
                            name: 'test1',
                            releases: [
                                {
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    undeployVersion: false,
                                    sourceCommitId: 'commit123',
                                    sourceAuthor: 'author',
                                    prNumber: '666',
                                    createdAt: new Date(2023, 6, 6),
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {},
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {},
        },
        {
            name: 'using a full release - component test',
            props: { app: 'test2', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    undeployVersion: false,
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    sourceCommitId: '12s3',
                                    sourceAuthor: 'test-author',
                                    prNumber: '666',
                                    createdAt: new Date(2002),
                                    displayVersion: '2',
                                    isMinor: true,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {},
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            },
            rels: [
                {
                    undeployVersion: false,
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: '12s3',
                    sourceAuthor: 'test-author',
                    prNumber: '666',
                    createdAt: new Date(2002),
                    displayVersion: '2',
                    isMinor: true,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {},
        },
        {
            name: 'using a deployed release - useDeployedAt test',
            props: { app: 'test2', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    undeployVersion: false,
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    sourceCommitId: '12s3',
                                    sourceAuthor: 'test-author',
                                    prNumber: '666',
                                    createdAt: new Date(2002),
                                    displayVersion: '2',
                                    isMinor: true,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {
                            foo: {
                                version: 2,
                                revision: 0,
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
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'test-author',
                    prNumber: '666',
                    undeployVersion: false,
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {
                foo: {
                    name: 'foo',
                    distanceToUpstream: 0,
                    priority: 0,
                },
            },
        },
        {
            name: 'using an undeployed release - useDeployedAt test',
            props: { app: 'test2', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    undeployVersion: false,
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    sourceCommitId: '12s3',
                                    sourceAuthor: 'test-author',
                                    prNumber: '666',
                                    createdAt: new Date(2002),
                                    displayVersion: '2',
                                    isMinor: true,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {
                            foo: {
                                version: 2,
                                revision: 0,
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
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: 'commit123',
                    undeployVersion: false,
                    createdAt: new Date(2023, 6, 6),
                    sourceAuthor: 'test-author',
                    prNumber: '666',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {
                undeployed: {
                    name: 'undeployed',
                    distanceToUpstream: 0,
                    priority: 0,
                },
            },
        },
        {
            name: 'using another environment - useDeployedAt test',
            props: { app: 'test2', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    sourceCommitId: 'commit123',
                                    undeployVersion: false,
                                    sourceAuthor: 'test-author',
                                    prNumber: '666',
                                    createdAt: new Date(2023, 6, 6),
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {
                            foo: {
                                version: 2,
                                revision: 0,
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
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: 'commit123',
                    undeployVersion: false,
                    sourceAuthor: 'test-author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {
                other: {
                    distanceToUpstream: 0,
                    priority: 0,
                    name: 'other',
                },
            },
        },
        {
            name: 'using a prepublished release',
            props: { app: 'test2', versionInfo: { version: 2, revision: 0 } },
            appDetails: {
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: [
                                {
                                    undeployVersion: false,
                                    version: 2,
                                    sourceMessage: 'test-rel',
                                    sourceCommitId: '12s3',
                                    sourceAuthor: 'test-author',
                                    prNumber: '666',
                                    createdAt: new Date(2002),
                                    displayVersion: '2',
                                    isMinor: true,
                                    isPrepublish: true,
                                    environments: [],
                                    ciLink: '',
                                    revision: 0,
                                },
                            ],
                            team: 'test-team',
                            sourceRepoUrl: '',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {
                            foo: {
                                version: 2,
                                revision: 0,
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
            rels: [
                {
                    undeployVersion: false,
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: '12s3',
                    sourceAuthor: 'test-author',
                    prNumber: '666',
                    createdAt: new Date(2002),
                    displayVersion: '2',
                    isMinor: true,
                    isPrepublish: true,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environments: {},
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                environmentGroups: [],
            });
            updateAppDetails.set(testcase.appDetails);
            const { container } = getWrapper(testcase.props);

            // then
            if (testcase.rels[0].undeployVersion) {
                expect(container.querySelector('.release__title')?.textContent).toContain('Undeploy Version');
            } else {
                expect(container.querySelector('.release__title')?.textContent).toContain(
                    testcase.rels[0].sourceMessage
                );
            }

            if (testcase.rels[0].isPrepublish) {
                expect(container.querySelector('.release-card__prepublish')).toBeInTheDocument();
                expect(container.querySelector('.release__title__prepublish')?.textContent).toContain(
                    testcase.rels[0].sourceMessage
                );
            }

            if (testcase.rels[0].isMinor) {
                expect(container.querySelector('.release__title')?.textContent).toContain('💤');
            }

            if (testcase.rels[0].displayVersion) {
                expect(container.querySelector('.release-version__display-version')?.textContent).toContain(
                    testcase.rels[0].displayVersion
                );
            } else if (testcase.rels[0].sourceCommitId) {
                expect(container.querySelector('.release-version__commit-id')?.textContent).toContain(
                    testcase.rels[0].sourceCommitId
                );
            }
            expect(container.querySelector('.env-group-chip-list-test')).not.toBeEmptyDOMElement();
            expect(container.querySelector('.release__status')).toBeNull();
        });
    });
});

const commonAppDetails: { [key: string]: AppDetailsResponse } = {
    test1: {
        details: {
            application: {
                name: '',
                releases: [
                    {
                        version: 2,
                        sourceMessage: 'test-rel',
                        undeployVersion: false,
                        sourceCommitId: 'commit123',
                        sourceAuthor: 'author',
                        prNumber: '666',
                        createdAt: new Date(2023, 6, 6),
                        displayVersion: '2',
                        isMinor: false,
                        isPrepublish: false,
                        environments: [],
                        ciLink: '',
                        revision: 0,
                    },
                ],
                team: 'test-team',
                sourceRepoUrl: '',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            deployments: {
                development: {
                    version: 2,
                    revision: 0,
                    queuedVersion: 0,
                    undeployVersion: false,
                },
                development2: {
                    version: 2,
                    revision: 0,
                    queuedVersion: 0,
                    undeployVersion: false,
                },
                staging: {
                    version: 2,
                    revision: 0,
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
};

describe('Release Card Rollout Status', () => {
    const getNode = (overrides: ReleaseCardProps) => (
        <MemoryRouter>
            <ReleaseCard {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseCardProps) => render(getNode(overrides));

    type TestData = {
        name: string;
        props: {
            app: string;
            versionInfo: ReleaseNumbers;
        };
        rels: Release[];
        environmentGroups: EnvironmentGroup[];
        rolloutStatus: StreamStatusResponse[];
        expectedStatusIcon: RolloutStatus;
        expectedRolloutDetails: { [name: string]: RolloutStatus };
        appDetails: { [key: string]: AppDetailsResponse };
    };
    const data: TestData[] = [
        {
            name: 'shows success when it is deployed',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: commonAppDetails,
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
                {
                    environmentGroupName: 'staging',
                    environments: [
                        {
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
            ],
            rolloutStatus: [
                {
                    environment: 'development',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'development2',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'staging',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
            ],

            expectedStatusIcon: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
            expectedRolloutDetails: {
                dev: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                staging: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
            },
        },
        {
            name: 'shows most interesting status with AA environments',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: commonAppDetails,
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
                {
                    environmentGroupName: 'staging',
                    environments: [
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
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
            ],
            rolloutStatus: [
                {
                    environment: 'development',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'development2',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'aa-staging-test-1',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                },
                {
                    environment: 'aa-staging-test-2',
                    application: 'test1',
                    version: 2,
                    rolloutStatus: RolloutStatus.ROLLOUT_STATUS_ERROR,
                },
            ],

            expectedStatusIcon: RolloutStatus.ROLLOUT_STATUS_ERROR,
            expectedRolloutDetails: {
                dev: RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
                staging: RolloutStatus.ROLLOUT_STATUS_ERROR,
            },
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                environmentGroups: testcase.environmentGroups,
            });
            updateAppDetails.set(testcase.appDetails);

            testcase.rolloutStatus.forEach(UpdateRolloutStatus);
            const { container } = getWrapper(testcase.props);
            // then
            expect(container.querySelector('.release__status')).not.toBeNull();
            expect(
                container.querySelector(
                    `.release__status .rollout__icon_${rolloutStatusName(testcase.expectedStatusIcon)}`
                )
            ).not.toBeNull();
            for (const [envGroup, status] of Object.entries(testcase.expectedRolloutDetails)) {
                const row = container.querySelector(`tr[key="${envGroup}"]`);
                expect(row?.querySelector(`.rollout__description_${rolloutStatusName(status)}`)).not.toBeNull();
            }
        });
    });
});

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

const gitSyncStatusNames = (status: GitSyncStatus): string => {
    switch (status) {
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCED:
            return 'successful';
        case GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED:
            return 'progressing';
        case GitSyncStatus.GIT_SYNC_STATUS_ERROR:
            return 'error';
        default:
            return 'unknown';
    }
};

describe('Release Card Git Sync Status', () => {
    const getNode = (overrides: ReleaseCardProps) => (
        <MemoryRouter>
            <ReleaseCard {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseCardProps) => render(getNode(overrides));

    type TestData = {
        name: string;
        props: {
            app: string;
            versionInfo: ReleaseNumbers;
        };
        rels: Release[];
        environmentGroups: EnvironmentGroup[];
        gitSyncStatus: GetGitSyncStatusResponse;
        expectedStatusIcon: GitSyncStatus;
        expectedGitSyncStatusDetails: { [name: string]: GitSyncStatus };
        appDetails: { [key: string]: AppDetailsResponse };
    };
    const data: TestData[] = [
        {
            name: 'shows success when it is deployed',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: commonAppDetails,
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
                {
                    environmentGroupName: 'staging',
                    environments: [
                        {
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
            ],
            gitSyncStatus: {
                appStatuses: {},
            },

            expectedStatusIcon: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
            expectedGitSyncStatusDetails: {
                dev: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
                staging: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
            },
        },
        {
            name: 'shows progressing when it is unsynced',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: commonAppDetails,
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
                {
                    environmentGroupName: 'staging',
                    environments: [
                        {
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
            ],
            gitSyncStatus: {
                appStatuses: {
                    test1: {
                        envStatus: {
                            staging: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
                        },
                    },
                },
            },

            expectedStatusIcon: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
            expectedGitSyncStatusDetails: {
                dev: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
                staging: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
            },
        },
        {
            name: 'shows error when it is sync failed',
            props: { app: 'test1', versionInfo: { version: 2, revision: 0 } },
            appDetails: commonAppDetails,
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    sourceAuthor: 'author',
                    prNumber: '666',
                    createdAt: new Date(2023, 6, 6),
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                    ciLink: '',
                    revision: 0,
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
                {
                    environmentGroupName: 'staging',
                    environments: [
                        {
                            name: 'staging',
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    distanceToUpstream: 0,
                },
            ],
            gitSyncStatus: {
                appStatuses: {
                    test1: {
                        envStatus: {
                            staging: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
                            development: GitSyncStatus.GIT_SYNC_STATUS_ERROR,
                        },
                    },
                },
            },

            expectedStatusIcon: GitSyncStatus.GIT_SYNC_STATUS_ERROR,
            expectedGitSyncStatusDetails: {
                dev: GitSyncStatus.GIT_SYNC_STATUS_ERROR,
                staging: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
            },
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                environmentGroups: testcase.environmentGroups,
            });
            updateAppDetails.set(testcase.appDetails);

            UpdateGitSyncStatus(testcase.gitSyncStatus);
            const { container } = getWrapper(testcase.props);
            // then
            expect(container.querySelector('.release__status')).not.toBeNull();
            expect(
                container.querySelector(
                    `.release__status .rollout__icon_${gitSyncStatusNames(testcase.expectedStatusIcon)}`
                )
            ).not.toBeNull();
            for (const [envGroup, status] of Object.entries(testcase.expectedGitSyncStatusDetails)) {
                const row = container.querySelector(`tr[key="${envGroup}"]`);
                expect(row?.querySelector(`.rollout__description_${gitSyncStatusNames(status)}`)).not.toBeNull();
            }
        });
    });
});
