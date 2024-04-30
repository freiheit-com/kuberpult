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
import { ReleaseCard, ReleaseCardProps } from './ReleaseCard';
import { render } from '@testing-library/react';
import { UpdateOverview, UpdateRolloutStatus } from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';
import {
    Environment,
    EnvironmentGroup,
    Priority,
    Release,
    RolloutStatus,
    StreamStatusResponse,
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
            version: number;
        };
        rels: Release[];
        environments: { [key: string]: Environment };
    };
    const data: TestData[] = [
        {
            name: 'using a sample release - useRelease hook',
            props: { app: 'test1', version: 2 },
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
                },
            ],
            environments: {},
        },
        {
            name: 'using a full release - component test',
            props: { app: 'test2', version: 2 },
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
                },
            ],
            environments: {},
        },
        {
            name: 'using a deployed release - useDeployedAt test',
            props: { app: 'test2', version: 2 },
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
                },
            ],
            environments: {
                foo: {
                    name: 'foo',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        test2: {
                            version: 2,
                            queuedVersion: 0,
                            name: 'test2',
                            locks: {},
                            teamLocks: {},
                            team: 'test-team',
                            undeployVersion: false,
                        },
                    },
                },
            },
        },
        {
            name: 'using an undeployed release - useDeployedAt test',
            props: { app: 'test2', version: 2 },
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
                },
            ],
            environments: {
                undeployed: {
                    name: 'undeployed',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        test2: {
                            version: 3,
                            queuedVersion: 0,
                            name: 'test2',
                            locks: {},
                            teamLocks: {},
                            team: 'test-team',
                            undeployVersion: false,
                        },
                    },
                },
            },
        },
        {
            name: 'using another environment - useDeployedAt test',
            props: { app: 'test2', version: 2 },
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
                },
            ],
            environments: {
                other: {
                    locks: {},

                    distanceToUpstream: 0,
                    priority: 0,
                    name: 'other',
                    applications: {
                        test3: {
                            version: 3,
                            queuedVersion: 0,
                            name: 'test2',
                            locks: {},
                            teamLocks: {},
                            team: 'test-team',
                            undeployVersion: false,
                        },
                    },
                },
            },
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                applications: {
                    [testcase.props.app]: {
                        name: testcase.props.app,
                        releases: testcase.rels,
                        sourceRepoUrl: 'url',
                        undeploySummary: UndeploySummary.NORMAL,
                        team: 'no-team',
                        warnings: [],
                    },
                },
                environmentGroups: [],
            });
            const { container } = getWrapper(testcase.props);

            // then
            if (testcase.rels[0].undeployVersion) {
                expect(container.querySelector('.release__title')?.textContent).toContain('Undeploy Version');
            } else {
                expect(container.querySelector('.release__title')?.textContent).toContain(
                    testcase.rels[0].sourceMessage
                );
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
            version: number;
        };
        rels: Release[];
        environmentGroups: EnvironmentGroup[];
        rolloutStatus: StreamStatusResponse[];
        expectedStatusIcon: RolloutStatus;
        expectedRolloutDetails: { [name: string]: RolloutStatus };
    };
    const data: TestData[] = [
        {
            name: 'shows success when it is deployed',
            props: { app: 'test1', version: 2 },
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
                },
            ],
            environmentGroups: [
                {
                    environmentGroupName: 'dev',
                    environments: [
                        {
                            name: 'development',
                            applications: {
                                test1: {
                                    version: 2,
                                    name: '',
                                    locks: {},
                                    teamLocks: {},
                                    team: 'test-team',
                                    queuedVersion: 0,
                                    undeployVersion: false,
                                },
                            },
                            locks: {},
                            distanceToUpstream: 0,
                            priority: Priority.OTHER,
                        },
                        {
                            name: 'development2',
                            applications: {
                                test1: {
                                    version: 2,
                                    name: '',
                                    locks: {},
                                    teamLocks: {},
                                    team: 'test-team',
                                    queuedVersion: 0,
                                    undeployVersion: false,
                                },
                            },
                            locks: {},
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
                            applications: {
                                test1: {
                                    version: 2,
                                    name: '',
                                    locks: {},
                                    teamLocks: {},
                                    team: 'test-team',
                                    queuedVersion: 0,
                                    undeployVersion: false,
                                },
                            },
                            locks: {},
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
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                applications: {
                    [testcase.props.app]: {
                        name: testcase.props.app,
                        releases: testcase.rels,
                        sourceRepoUrl: 'url',
                        undeploySummary: UndeploySummary.NORMAL,
                        team: 'no-team',
                        warnings: [],
                    },
                },
                environmentGroups: testcase.environmentGroups,
            });
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
