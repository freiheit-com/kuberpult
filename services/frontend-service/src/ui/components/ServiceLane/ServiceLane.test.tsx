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
import { fireEvent, render } from '@testing-library/react';
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

const mock_ReleaseCard = Spy.mockReactComponents('../../components/ReleaseCard/ReleaseCard', 'ReleaseCard');
const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');
const sampleEnvs = {
    foo: {
        // third release card contains two environments
        name: 'foo',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    foo2: {
        // third release card contains two environments
        name: 'foo2',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    bar: {
        // second release card contains one environment, newest version
        name: 'bar',
        applications: {
            test2: {
                version: 3,
            },
        },
    },
    undeploy: {
        // first release card is for the undeploy one
        name: 'undeploy',
        applications: {
            test2: {
                version: 5,
                undeployVersion: true,
            },
        },
    },
    other: {
        // no release card for different app
        name: 'other',
        applications: {
            test3: {
                version: 3,
            },
        },
    },
};

describe('Service Lane', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        const sampleApp = {
            name: 'test2',
            releases: [{ version: 5 }, { version: 2 }, { version: 3 }],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
            undeploySummary: '',
        };
        UpdateOverview.set({
            environments: sampleEnvs as any,
            applications: {
                test2: sampleApp as any,
            },
        });
        getWrapper({ application: sampleApp as any });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({ app: sampleApp.name, version: 5 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({ app: sampleApp.name, version: 3 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({ app: sampleApp.name, version: 2 });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });
});

const getMockRelease = (version: number): Release => ({
    version: version,
    sourceMessage: 'test' + version,
    sourceAuthor: 'test',
    sourceCommitId: 'commit' + version,
    createdAt: new Date(2002),
    undeployVersion: false,
    prNumber: '666',
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
        releases: [getMockRelease(1)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
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
        releases: [getMockRelease(1), getMockRelease(2)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
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
        releases: [getMockRelease(1), getMockRelease(4), getMockRelease(2)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        locks: {},
                    },
                },
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 4,
                        locks: {},
                    },
                },
                locks: {},
            } as any,
        ],
    },
    {
        name: 'test diff by two',
        diff: '2',
        releases: [getMockRelease(2), getMockRelease(4), getMockRelease(3), getMockRelease(5)],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 2,
                        name: '',
                        locks: {},
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
                environments: undefined as any, // deprecated
                applications: {
                    test2: {
                        releases: testcase.releases,
                        name: '',
                        team: '',
                        sourceRepoUrl: '',
                        undeploySummary: UndeploySummary.Mixed,
                    },
                },
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                    },
                ],
            });
            const sampleApp: Application = {
                undeploySummary: UndeploySummary.Normal,
                name: 'test2',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
            };
            const { container } = getWrapper({ application: sampleApp as any });

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
            getMockRelease(9),
            getMockRelease(7),
            getMockRelease(6),
            getMockRelease(5),
            getMockRelease(4),
            getMockRelease(3),
            getMockRelease(2),
            getMockRelease(1),
        ],
    },
    {
        name: 'Gets latest release first, then deployed release and 4 trailing releases',
        currentlyDeployedVersion: 7,
        releases: [
            getMockRelease(9),
            getMockRelease(7),
            getMockRelease(6),
            getMockRelease(5),
            getMockRelease(4),
            getMockRelease(3),
            getMockRelease(2),
            getMockRelease(1),
        ],
    },
    {
        name: 'jumps over not important second release',
        currentlyDeployedVersion: 6,
        releases: [
            getMockRelease(9),
            getMockRelease(7), // not important
            getMockRelease(6),
            getMockRelease(5),
            getMockRelease(4),
            getMockRelease(3),
            getMockRelease(2),
            getMockRelease(1),
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
                undeploySummary: UndeploySummary.Mixed,
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
                undeploySummary: UndeploySummary.Normal,
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
            expectedUndeployButton: 'Prepare Undeploy Release',
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
                undeploySummary: UndeploySummary.Undeploy,
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
            expectedUndeployButton: 'Delete Forever',
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

describe('Service Lane Undeploy Buttons', () => {
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
                    },
                ],
            });

            const { container } = getWrapper({ application: testcase.renderedApp });

            const undeployButton = container.querySelector('.service-action.service-action--prepare-undeploy');
            const label = undeployButton?.querySelector('span');
            expect(label?.textContent).toBe(testcase.expectedUndeployButton);

            mock_addAction.addAction.wasNotCalled();

            fireEvent.click(undeployButton!);

            mock_addAction.addAction.wasCalledWith(testcase.expectedAction);
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
    };
    const result: TestDataAppLockSummary[] = [
        {
            name: 'test no prepareUndeploy',
            renderedApp: {
                name: 'test1',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.Normal,
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
                undeploySummary: UndeploySummary.Normal,
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
                undeploySummary: UndeploySummary.Normal,
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
                    },
                ],
            });

            const { container } = getWrapper({ application: testcase.renderedApp });

            const appLockSummary = container.querySelector('.test-app-lock-summary div');
            expect(appLockSummary?.attributes.getNamedItem('aria-label')?.value).toBe(testcase.expected);
        });
    });
});
