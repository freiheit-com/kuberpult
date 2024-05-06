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
import { render } from '@testing-library/react';
import { ServiceLane } from './ServiceLane';
import { UpdateOverview } from '../../utils/store';
import { Spy } from 'spy4js';
import {
    Application,
    BatchAction,
    Environment,
    Environment_Application,
    Priority,
    Release,
    UndeploySummary,
} from '../../../api/api';
import { MemoryRouter } from 'react-router-dom';
import { elementQuerySelectorSafe, makeRelease } from '../../../setupTests';

const mock_ReleaseCard = Spy.mockReactComponents('../../components/ReleaseCard/ReleaseCard', 'ReleaseCard');
const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

const extendRelease = (props: Partial<Release>): Release => ({
    version: 123,
    displayVersion: '123',
    sourceCommitId: 'id',
    sourceAuthor: 'author',
    sourceMessage: 'source',
    undeployVersion: false,
    prNumber: 'pr',
    ...props,
});

describe('Service Lane', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        const sampleApp: Application = {
            name: 'test2',
            releases: [extendRelease({ version: 5 }), extendRelease({ version: 2 }), extendRelease({ version: 3 })],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
            undeploySummary: UndeploySummary.NORMAL,
            warnings: [],
        };
        UpdateOverview.set({
            applications: {
                test2: sampleApp,
            },
        });
        getWrapper({ application: sampleApp });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({ app: sampleApp.name, version: 5 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({ app: sampleApp.name, version: 3 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({ app: sampleApp.name, version: 2 });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });
});

type TestData = {
    name: string;
    envs: Environment[];
};

type TestDataDiff = TestData & { diff: string; releases: Release[] };

const data: TestDataDiff[] = [
    {
        name: 'test same version',
        diff: '-1',
        releases: [makeRelease(1)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
    {
        name: 'test no diff',
        diff: '0',
        releases: [makeRelease(1), makeRelease(2)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 2,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
    {
        name: 'test diff by one',
        diff: '1',
        releases: [makeRelease(1), makeRelease(4), makeRelease(2)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        name: 'test2',
                        version: 1,
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                locks: {},
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        name: 'test2',
                        version: 4,
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                locks: {},
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
        ],
    },
    {
        name: 'test diff by two',
        diff: '2',
        releases: [makeRelease(2), makeRelease(4), makeRelease(3), makeRelease(5)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 2,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 5,
                        name: '',
                        locks: {},
                        teamLocks: {},
                        team: 'test-team',
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
];

describe('Service Lane Diff', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    describe.each(data)('Service Lane diff number', (testcase) => {
        it(testcase.name, () => {
            UpdateOverview.set({
                applications: {
                    test2: {
                        releases: testcase.releases,
                        name: '',
                        team: '',
                        sourceRepoUrl: '',
                        undeploySummary: UndeploySummary.MIXED,
                        warnings: [],
                    },
                },
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            const sampleApp: Application = {
                undeploySummary: UndeploySummary.NORMAL,
                name: 'test2',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                warnings: [],
            };
            const { container } = getWrapper({ application: sampleApp });

            // check for the diff between versions
            if (testcase.diff === '-1' || testcase.diff === '0') {
                expect(document.querySelector('.service-lane__diff--number') === undefined);
            } else {
                expect(container.querySelector('.service-lane__diff--number')?.textContent).toContain(testcase.diff);
            }
        });
    });
});

type TestDataImportantRels = { name: string; releases: Release[]; currentlyDeployedVersion: number };

const dataImportantRels: TestDataImportantRels[] = [
    {
        name: 'Gets deployed release first and 5 trailing releases',
        currentlyDeployedVersion: 9,
        releases: [
            makeRelease(9),
            makeRelease(7),
            makeRelease(6),
            makeRelease(5),
            makeRelease(4),
            makeRelease(3),
            makeRelease(2),
            makeRelease(1), // not important
        ],
    },
    {
        name: 'Gets latest release first, then deployed release and 4 trailing releases',
        currentlyDeployedVersion: 7,
        releases: [
            makeRelease(9),
            makeRelease(7),
            makeRelease(6),
            makeRelease(5),
            makeRelease(4),
            makeRelease(3),
            makeRelease(2),
            makeRelease(1), // not important
        ],
    },
    {
        name: 'jumps over not important second release',
        currentlyDeployedVersion: 6,
        releases: [
            makeRelease(9),
            makeRelease(7), // not important
            makeRelease(6),
            makeRelease(5),
            makeRelease(4),
            makeRelease(3),
            makeRelease(2),
            makeRelease(1), // not important
        ],
    },
];

describe('Service Lane Important Releases', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    describe.each(dataImportantRels)('Service Lane important releases', (testcase) => {
        it(testcase.name, () => {
            // given
            const sampleApp: Application = {
                releases: testcase.releases,
                name: 'test2',
                team: 'test2',
                sourceRepoUrl: 'test2',
                undeploySummary: UndeploySummary.MIXED,
                warnings: [],
            };
            UpdateOverview.set({
                applications: {
                    test2: sampleApp,
                },
                environmentGroups: [
                    {
                        environments: [
                            {
                                name: 'foo',
                                applications: {
                                    test2: {
                                        name: 'test2',
                                        version: testcase.currentlyDeployedVersion,
                                        locks: {},
                                        teamLocks: {},
                                        team: 'test-team',
                                        undeployVersion: false,
                                        queuedVersion: 0,
                                    },
                                },
                                distanceToUpstream: 0,
                                priority: Priority.UPSTREAM,
                                locks: {},
                            },
                        ],
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            // when
            getWrapper({ application: sampleApp });

            // then - the latest release is always important and is displayed first
            expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0)).toMatchObject({
                version: testcase.releases[0].version,
            });
            if (testcase.currentlyDeployedVersion !== testcase.releases[0].version) {
                // then - the currently deployed version always important and displayed second after latest
                expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1)).toMatchObject({
                    version: testcase.currentlyDeployedVersion,
                });
            }
            if (testcase.releases[1].version > testcase.currentlyDeployedVersion) {
                // then - second release not deployed and not latest -> not important
                mock_ReleaseCard.ReleaseCard.wasNotCalledWith(
                    { app: 'test2', version: testcase.releases[1].version },
                    Spy.IGNORE
                );
            }
            // then - the old release is not important and not displayed
            mock_ReleaseCard.ReleaseCard.wasNotCalledWith(
                { app: 'test2', version: testcase.releases[7].version },
                Spy.IGNORE
            );
        });
    });
});

type TestDataUndeploy = TestData & {
    renderedApp: Application;
    expectedUndeployButton: string | null;
    expectedAction: BatchAction;
};
const dataUndeploy: TestDataUndeploy[] = (() => {
    const result: TestDataUndeploy[] = [
        {
            name: 'test no prepareUndeploy',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            envs: [
                {
                    name: 'foo2',
                    applications: {},
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                    locks: {},
                },
            ],
            expectedUndeployButton: '⋮',
            expectedAction: {
                action: {
                    $case: 'prepareUndeploy',
                    prepareUndeploy: { application: 'test1' },
                },
            },
        },
        {
            name: 'test no undeploy',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.UNDEPLOY,
                warnings: [],
            },
            envs: [
                {
                    name: 'foo2',
                    applications: {},
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                    locks: {},
                },
            ],
            expectedUndeployButton: '⋮',
            expectedAction: {
                action: {
                    $case: 'undeploy',
                    undeploy: { application: 'test1' },
                },
            },
        },
    ];
    return result;
})();

describe('Service Lane ⋮ menu', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    describe.each(dataUndeploy)('Undeploy Buttons', (testcase) => {
        it(testcase.name, () => {
            mock_addAction.addAction.returns(undefined);

            UpdateOverview.set({
                applications: {
                    test1: testcase.renderedApp,
                },
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            const { container } = getWrapper({ application: testcase.renderedApp });

            const undeployButton = elementQuerySelectorSafe(container, '.dots-menu-hidden');
            const label = elementQuerySelectorSafe(undeployButton, 'span');
            expect(label?.textContent).toBe(testcase.expectedUndeployButton);

            mock_addAction.addAction.wasNotCalled();
        });
    });
});

type TestDataAppLockSummary = TestData & {
    renderedApp: Application;
    expected: string | undefined;
};
const dataAppLockSummary: TestDataAppLockSummary[] = (() => {
    const appWith1Lock: Environment_Application = {
        name: 'test1',
        version: 123,
        queuedVersion: 0,
        undeployVersion: false,
        locks: {
            l1: { message: 'test lock', lockId: '321' },
        },
        teamLocks: {},
        team: 'test-team',
    };
    const appWith2Locks: Environment_Application = {
        name: 'test1',
        version: 123,
        queuedVersion: 0,
        undeployVersion: false,
        locks: {
            l1: { message: 'test lock', lockId: '321' },
            l2: { message: 'test lock', lockId: '321' },
        },
        teamLocks: {},
        team: 'test-team',
    };
    const result: TestDataAppLockSummary[] = [
        {
            name: 'test no prepareUndeploy',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            envs: [
                {
                    name: 'foo2',
                    applications: {},
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                    locks: {
                        envLockThatDoesNotMatter: {
                            message: 'I am an env lock, I should not count',
                            lockId: '487329463874223',
                        },
                    },
                },
            ],
            expected: undefined,
        },
        {
            name: 'test one lock',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            envs: [
                {
                    name: 'foo2',
                    applications: {
                        foo2: appWith1Lock,
                    },
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                    locks: {},
                },
            ],
            expected: '"test1" has 1 application lock. Click on a tile to see details.',
        },
        {
            name: 'test two locks',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            envs: [
                {
                    name: 'foo2',
                    applications: {
                        foo2: appWith2Locks,
                    },
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                    locks: {},
                },
            ],
            expected: '"test1" has 2 application locks. Click on a tile to see details.',
        },
    ];
    return result;
})();

describe('Service Lane AppLockSummary', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    describe.each(dataAppLockSummary)('diff', (testcase) => {
        it(testcase.name, () => {
            mock_addAction.addAction.returns(undefined);

            UpdateOverview.set({
                applications: {
                    test1: testcase.renderedApp,
                },
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            const { container } = getWrapper({ application: testcase.renderedApp });

            const appLockSummary = container.querySelector('.test-app-lock-summary div');
            expect(appLockSummary?.attributes.getNamedItem('title')?.value).toBe(testcase.expected);
        });
    });
});
