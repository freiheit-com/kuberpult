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
import { fireEvent, render, screen } from '@testing-library/react';
import { DiffElement, GeneralServiceLane, ServiceLane } from './ServiceLane';
import {
    AppDetailsResponse,
    AppDetailsState,
    ReleaseNumbers,
    updateAppDetails,
    UpdateOverview,
} from '../../utils/store';
import { Spy } from 'spy4js';
import {
    Application,
    BatchAction,
    Environment,
    GetAppDetailsResponse,
    OverviewApplication,
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
    isMinor: false,
    isPrepublish: false,
    environments: [],
    ciLink: '',
    revision: 0,
    ...props,
});

describe('Service Lane', () => {
    const getNode = (overrides: {
        application: OverviewApplication;
        allAppDetails: { [p: string]: AppDetailsResponse };
    }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} hideMinors={false} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: {
        application: OverviewApplication;
        allAppDetails: { [p: string]: AppDetailsResponse };
    }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        const appDetails = {
            test3: {
                details: {
                    application: {
                        name: 'test3',
                        releases: [
                            extendRelease({ version: 2, revision: 0 }),
                            extendRelease({ version: 3, revision: 0 }),
                            extendRelease({ version: 5, revision: 0 }),
                        ],
                        sourceRepoUrl: 'http://test2.com',
                        team: 'example',
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
        };
        const sampleApp: Application = {
            name: 'test3',
            releases: [
                extendRelease({ version: 5, revision: 0 }),
                extendRelease({ version: 2, revision: 0 }),
                extendRelease({ version: 3, revision: 0 }),
            ],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
            undeploySummary: UndeploySummary.NORMAL,
            warnings: [],
        };
        const sampleLightWeightApp: OverviewApplication = {
            name: 'test3',
            team: 'example',
        };
        UpdateOverview.set({});
        updateAppDetails.set(appDetails);
        getWrapper({ application: sampleLightWeightApp, allAppDetails: appDetails });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 5,
                revision: 0,
            },
        });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 3,
                revision: 0,
            },
        });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 2,
                revision: 0,
            },
        });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });

    it('Renders a row of releases with revisions', () => {
        // when
        const appDetails = {
            test3: {
                details: {
                    application: {
                        name: 'test3',
                        releases: [
                            extendRelease({ version: 1, revision: 2 }),
                            extendRelease({ version: 1, revision: 5 }),
                            extendRelease({ version: 1, revision: 4 }),
                            extendRelease({ version: 2, revision: 0 }),
                        ],
                        sourceRepoUrl: 'http://test2.com',
                        team: 'example',
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
        };
        const sampleApp: Application = {
            name: 'test3',
            releases: [
                extendRelease({ version: 1, revision: 2 }),
                extendRelease({ version: 1, revision: 5 }),
                extendRelease({ version: 1, revision: 4 }),
                extendRelease({ version: 2, revision: 0 }),
            ],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
            undeploySummary: UndeploySummary.NORMAL,
            warnings: [],
        };
        const sampleLightWeightApp: OverviewApplication = {
            name: 'test3',
            team: 'example',
        };
        UpdateOverview.set({});
        updateAppDetails.set(appDetails);
        getWrapper({ application: sampleLightWeightApp, allAppDetails: appDetails });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 2,
                revision: 0,
            },
        });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 1,
                revision: 5,
            },
        });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 1,
                revision: 4,
            },
        });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(3, 0)).toStrictEqual({
            app: sampleApp.name,
            versionInfo: {
                version: 1,
                revision: 2,
            },
        });
        mock_ReleaseCard.ReleaseCard.wasCalled(4);
    });
});

type TestDataServiceLaneState = { name: string; appDetails: AppDetailsResponse };

const serviceLaneStates: TestDataServiceLaneState[] = [
    {
        name: 'Ready',
        appDetails: {
            details: {
                application: {
                    name: 'test-ready',
                    team: 'test-team',
                    releases: [makeRelease(1)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.READY,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
    },
    {
        name: 'loading',
        appDetails: {
            details: {
                application: {
                    name: 'test-loading',
                    team: 'test-team',
                    releases: [makeRelease(4), makeRelease(2), makeRelease(1)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 4,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.LOADING,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
    },
    {
        name: 'NotFound',
        appDetails: {
            details: {
                application: {
                    name: 'test-not-found',
                    team: 'test-team',
                    releases: [makeRelease(2), makeRelease(3), makeRelease(4), makeRelease(5)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 2,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 5,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.NOTFOUND,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
    },

    {
        name: 'Error',
        appDetails: {
            details: {
                application: {
                    name: 'test-error',
                    team: 'test-team',
                    releases: [makeRelease(1)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {},
            },
            appDetailState: AppDetailsState.ERROR,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
    },
    {
        name: 'NotRequested',
        appDetails: {
            details: {
                application: {
                    name: 'test-not-requested',
                    team: 'test-team-testtest',
                    releases: [],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {},
            },
            appDetailState: AppDetailsState.NOTREQUESTED,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
    },
];

describe('Service Lane States', () => {
    const getNode = (overrides: {
        application: OverviewApplication;
        hideMinors: boolean;
        allAppData: AppDetailsResponse;
    }) => (
        <MemoryRouter>
            <GeneralServiceLane
                application={overrides.application}
                hideMinors={false}
                allAppData={overrides.allAppData}
            />
        </MemoryRouter>
    );
    const getWrapper = (overrides: {
        application: OverviewApplication;
        hideMinors: boolean;
        allAppData: AppDetailsResponse;
    }) => render(getNode(overrides));
    describe.each(serviceLaneStates)('Service Lane states', (testcase) => {
        it(testcase.name, () => {
            const sampleLightweightApp: OverviewApplication = {
                name: testcase.appDetails.details?.application?.name
                    ? testcase.appDetails.details?.application?.name
                    : '',
                team: testcase.appDetails.details?.application?.team
                    ? testcase.appDetails.details?.application?.team
                    : '',
            };

            const appDetails = updateAppDetails.get();
            appDetails[sampleLightweightApp.name] = testcase.appDetails;

            const { container } = getWrapper({
                application: sampleLightweightApp,
                hideMinors: false,
                allAppData: testcase.appDetails,
            });
            if (testcase.appDetails.appDetailState === AppDetailsState.NOTREQUESTED) {
                expect(container.querySelector('.service-lane')).toBeInTheDocument();
                expect(container.querySelector('.service-lane__header__not_requested')).toBeInTheDocument();
            } else if (testcase.appDetails.appDetailState === AppDetailsState.NOTFOUND) {
                expect(container.querySelector('.service-lane')).toBeInTheDocument();
                expect(container.querySelector('.service-lane__header__warn')).toBeInTheDocument();
            } else if (testcase.appDetails.appDetailState === AppDetailsState.ERROR) {
                expect(container.querySelector('.service-lane')).toBeInTheDocument();
                expect(container.querySelector('.service-lane__header__error')).toBeInTheDocument();
            } else {
                //READY and LOADING are represented the same way
                expect(container.querySelector('.service-lane')).toBeInTheDocument();
                expect(container.querySelector('.service-lane__header')).toBeInTheDocument();
            }
        });
    });
});

type TestData = {
    name: string;
    envs: Environment[];
};

type TestDataDiff = TestData & { diff: string; releases: Release[]; appDetails: AppDetailsResponse };

const data: TestDataDiff[] = [
    {
        name: 'test same version',
        diff: '-1',
        releases: [makeRelease(1)],
        appDetails: {
            details: {
                application: {
                    name: 'test2',
                    team: 'test-team',
                    releases: [makeRelease(1)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.READY,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
        envs: [
            {
                name: 'foo',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
            {
                name: 'foo2',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
        ],
    },
    {
        name: 'test no diff',
        diff: '0',
        releases: [makeRelease(1), makeRelease(2)],
        appDetails: {
            details: {
                application: {
                    name: 'test2',
                    team: 'test-team',
                    releases: [makeRelease(1), makeRelease(2)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 2,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.READY,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
        envs: [
            {
                name: 'foo',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
            {
                name: 'foo2',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
        ],
    },
    {
        name: 'test diff by one',
        diff: '1',
        releases: [makeRelease(1), makeRelease(2), makeRelease(4)],
        appDetails: {
            details: {
                application: {
                    name: 'test2',
                    team: 'test-team',
                    releases: [makeRelease(4), makeRelease(2), makeRelease(1)],
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 1,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 4,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.READY,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
        envs: [
            {
                name: 'foo',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
            {
                name: 'foo2',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
        ],
    },
    {
        name: 'test diff by two',
        diff: '2',
        releases: [makeRelease(2), makeRelease(4), makeRelease(3), makeRelease(5)],
        appDetails: {
            details: {
                application: {
                    name: 'test2',
                    team: 'test-team',
                    releases: [makeRelease(2), makeRelease(3), makeRelease(4), makeRelease(5)].reverse(),
                    sourceRepoUrl: '',
                    undeploySummary: UndeploySummary.MIXED,
                    warnings: [],
                },
                appLocks: {},
                teamLocks: {},
                deployments: {
                    foo: {
                        version: 2,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                    foo2: {
                        version: 5,
                        revision: 0,
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
            },
            appDetailState: AppDetailsState.READY,
            updatedAt: new Date(Date.now()),
            errorMessage: '',
        },
        envs: [
            {
                name: 'foo',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
            {
                name: 'foo2',
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
            },
        ],
    },
];

describe('Service Lane Diff', () => {
    const getNode = (overrides: { application: OverviewApplication }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} hideMinors={false} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: OverviewApplication }) => render(getNode(overrides));
    describe.each(data)('Service Lane diff number', (testcase) => {
        it(testcase.name, () => {
            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            updateAppDetails.set({
                test2: testcase.appDetails,
            });
            const sampleLightweightApp: OverviewApplication = {
                name: 'test2',
                team: 'test-team',
            };
            const { container } = getWrapper({ application: sampleLightweightApp });

            // check for the diff between versions
            if (testcase.diff === '-1' || testcase.diff === '0') {
                expect(document.querySelector('.service-lane__diff--number') === undefined);
            } else {
                expect(container.querySelector('.service-lane__diff--number')?.textContent).toContain(testcase.diff);
            }
        });
    });
});

type TestDataImportantRels = {
    name: string;
    releases: Release[];
    currentlyDeployedVersion: ReleaseNumbers;
    minorReleaseIndex: number;
};

const dataImportantRels: TestDataImportantRels[] = [
    {
        name: 'Gets deployed release first and 5 trailing releases',
        currentlyDeployedVersion: {
            version: 9,
            revision: 0,
        },
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
        minorReleaseIndex: 7,
    },
    {
        name: 'Gets latest release first, then deployed release and 4 trailing releases',
        currentlyDeployedVersion: {
            version: 7,
            revision: 0,
        },
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
        minorReleaseIndex: 7,
    },
    {
        name: 'jumps over not important second release',
        currentlyDeployedVersion: {
            version: 6,
            revision: 0,
        },
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
        minorReleaseIndex: 7,
    },
    {
        name: 'Minor release should be ignored',
        currentlyDeployedVersion: {
            version: 9,
            revision: 0,
        },
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
        minorReleaseIndex: 1,
    },
    {
        name: 'Test revisions',
        currentlyDeployedVersion: {
            version: 9,
            revision: 7,
        },
        releases: [
            makeRelease(9, '', '', false, 9),
            makeRelease(9, '', '', false, 7),
            makeRelease(9, '', '', false, 6),
            makeRelease(9, '', '', false, 5),
            makeRelease(9, '', '', false, 4),
            makeRelease(9, '', '', false, 3),
            makeRelease(9, '', '', false, 2),
            makeRelease(9, '', '', false, 1),
        ],
        minorReleaseIndex: 1,
    },
];

describe('Service Lane Important Releases', () => {
    const getNode = (overrides: { application: OverviewApplication }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} hideMinors={true} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: OverviewApplication }) => render(getNode(overrides));
    describe.each(dataImportantRels)('Service Lane important releases', (testcase) => {
        it(testcase.name, () => {
            // given
            testcase.releases[testcase.minorReleaseIndex].isMinor = true;
            const sampleApp: Application = {
                releases: testcase.releases,
                name: 'test2',
                team: 'test2',
                sourceRepoUrl: 'test2',
                undeploySummary: UndeploySummary.MIXED,
                warnings: [],
            };
            const sampleOverviewApp: OverviewApplication = {
                name: 'test2',
                team: 'test2',
            };
            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: [
                            {
                                name: 'foo',
                                distanceToUpstream: 0,
                                priority: Priority.UPSTREAM,
                            },
                        ],
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            updateAppDetails.set({
                test2: {
                    details: {
                        application: sampleApp,
                        deployments: {
                            foo: {
                                version: testcase.currentlyDeployedVersion.version,
                                revision: testcase.currentlyDeployedVersion.revision,
                                undeployVersion: false,
                                queuedVersion: 0,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            });
            // when
            getWrapper({ application: sampleOverviewApp });
            // then - the latest release is always important and is displayed first
            expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0)).toMatchObject({
                app: 'test2',
                versionInfo: {
                    version: testcase.releases[0].version,
                    revision: testcase.releases[0].revision,
                },
            });
            if (
                !(
                    testcase.currentlyDeployedVersion.version === testcase.releases[0].version &&
                    testcase.currentlyDeployedVersion.revision === testcase.releases[0].revision
                )
            ) {
                // then - the currently deployed version always important and displayed second after latest
                mock_ReleaseCard.ReleaseCard.wasNotCalledWith(
                    {
                        app: 'test2',
                        versionInfo: { version: testcase.releases[1].version, revison: testcase.releases[1].revision },
                    },
                    Spy.IGNORE
                );
            }
            if (
                testcase.releases[1].version > testcase.currentlyDeployedVersion.version ||
                (testcase.releases[1].version === testcase.currentlyDeployedVersion.version &&
                    testcase.releases[1].revision > testcase.currentlyDeployedVersion.revision)
            ) {
                // then - second release not deployed and not latest -> not important
                mock_ReleaseCard.ReleaseCard.wasNotCalledWith(Spy.IGNORE);
            }
            // then - the old release is not important and not displayed
            mock_ReleaseCard.ReleaseCard.wasNotCalledWith(
                {
                    app: 'test2',
                    versionInfo: { version: testcase.releases[7].version, revison: testcase.releases[7].revision },
                },
                Spy.IGNORE
            );
            mock_ReleaseCard.ReleaseCard.wasNotCalledWith(
                {
                    app: 'test2',
                    versionInfo: {
                        version: testcase.releases[testcase.minorReleaseIndex].version,
                        revison: testcase.releases[testcase.minorReleaseIndex].revision,
                    },
                },
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
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
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
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
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
    const getNode = (overrides: { application: OverviewApplication }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} hideMinors={false} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: OverviewApplication }) => render(getNode(overrides));
    describe.each(dataUndeploy)('Undeploy Buttons', (testcase) => {
        it(testcase.name, () => {
            mock_addAction.addAction.returns(undefined);

            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            updateAppDetails.set({
                test1: {
                    details: {
                        application: testcase.renderedApp,
                        deployments: {},
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            });

            const { container } = getWrapper({
                application: { name: testcase.renderedApp.name, team: testcase.renderedApp.team },
            });

            const undeployButton = elementQuerySelectorSafe(container, '.dots-menu-hidden');
            const label = elementQuerySelectorSafe(undeployButton, 'span');
            expect(label?.textContent).toBe(testcase.expectedUndeployButton);

            mock_addAction.addAction.wasNotCalled();
        });
    });
});

type TestDataAppLockSummary = TestData & {
    renderedApp: GetAppDetailsResponse;
    expected: string | undefined;
};
const dataAppLockSummary: TestDataAppLockSummary[] = (() => {
    const topLevelApp: Application = {
        name: 'test1',
        releases: [],
        sourceRepoUrl: 'http://test2.com',
        team: 'example',
        undeploySummary: UndeploySummary.NORMAL,
        warnings: [],
    };
    const appWith1AppLock: GetAppDetailsResponse = {
        application: topLevelApp,
        deployments: {
            foo2: {
                version: 123,
                revision: 0,
                queuedVersion: 0,
                undeployVersion: false,
            },
        },
        appLocks: {
            foo2: {
                locks: [{ message: 'test lock', lockId: '321', ciLink: '', suggestedLifetime: '' }],
            },
        },
        teamLocks: {},
    };
    const appWith1TeamLock: GetAppDetailsResponse = {
        application: topLevelApp,
        deployments: {
            foo2: {
                version: 123,
                revision: 0,
                queuedVersion: 0,
                undeployVersion: false,
            },
        },
        appLocks: {},
        teamLocks: {
            foo2: {
                locks: [{ message: 'test team lock', lockId: 't-1000', ciLink: '', suggestedLifetime: '' }],
            },
        },
    };
    const appWith1TeamLock1AppLock: GetAppDetailsResponse = {
        application: topLevelApp,
        deployments: {
            foo2: {
                version: 123,
                revision: 0,
                queuedVersion: 0,
                undeployVersion: false,
            },
        },
        appLocks: {
            foo2: {
                locks: [
                    { message: 'test lock', lockId: '321', ciLink: '', suggestedLifetime: '' },
                    { message: 'test app lock', lockId: 'a-1', ciLink: '', suggestedLifetime: '' },
                ],
            },
        },
        teamLocks: {
            foo2: {
                locks: [{ message: 'test team lock', lockId: 't-1000', ciLink: '', suggestedLifetime: '' }],
            },
        },
    };

    const appWith2Locks: GetAppDetailsResponse = {
        application: topLevelApp,
        deployments: {
            foo2: {
                version: 123,
                revision: 0,
                queuedVersion: 0,
                undeployVersion: false,
            },
        },
        appLocks: {
            foo2: {
                locks: [
                    { message: 'test lock', lockId: '321', ciLink: '', suggestedLifetime: '' },
                    { message: 'test lock', lockId: '321', ciLink: '', suggestedLifetime: '' },
                ],
            },
        },
        teamLocks: {
            foo2: {
                locks: [{ message: 'test team lock', lockId: 't-1000', ciLink: '', suggestedLifetime: '' }],
            },
        },
    };
    const result: TestDataAppLockSummary[] = [
        {
            name: 'test no prepareUndeploy',
            renderedApp: {
                application: {
                    name: 'test1',
                    releases: [],
                    sourceRepoUrl: 'http://test2.com',
                    team: 'example',
                    undeploySummary: UndeploySummary.NORMAL,
                    warnings: [],
                },
                deployments: {},
                teamLocks: {},
                appLocks: {},
            },
            envs: [
                {
                    name: 'foo2',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            expected: undefined,
        },
        {
            name: 'test one app lock',
            renderedApp: appWith1AppLock,
            envs: [
                {
                    name: 'foo2',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            expected: '"test1" has 1 lock. Click on a tile to see details.',
        },
        {
            name: 'test two app locks',
            renderedApp: appWith2Locks,
            envs: [
                {
                    name: 'foo2',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            expected: '"test1" has 2 locks. Click on a tile to see details.',
        },
        {
            name: 'test one team lock',
            renderedApp: appWith1TeamLock,
            envs: [
                {
                    name: 'foo2',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            expected: '"test1" has 1 lock. Click on a tile to see details.',
        },
        {
            name: 'test one team + one app lock',
            renderedApp: appWith1TeamLock1AppLock,
            envs: [
                {
                    name: 'foo2',
                    distanceToUpstream: 0,
                    priority: Priority.UPSTREAM,
                },
            ],
            expected: '"test1" has 2 locks. Click on a tile to see details.',
        },
    ];
    return result;
})();

describe('Service Lane AppLockSummary', () => {
    const getNode = (overrides: { application: OverviewApplication }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} hideMinors={false} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: OverviewApplication }) => render(getNode(overrides));
    describe.each(dataAppLockSummary)('diff', (testcase) => {
        it(testcase.name, () => {
            mock_addAction.addAction.returns(undefined);

            UpdateOverview.set({
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            updateAppDetails.set({
                test1: {
                    details: testcase.renderedApp,
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            });

            const { container } = getWrapper({
                application: {
                    name: testcase.renderedApp.application?.name || '',
                    team: testcase.renderedApp.application?.team || '',
                },
            });

            const appLockSummary = container.querySelector('.test-app-lock-summary div');
            expect(appLockSummary?.attributes.getNamedItem('title')?.value).toBe(testcase.expected);
        });
    });
});

test('Hidden commits button', () => {
    const testClick = jest.fn();
    render(<DiffElement diff={3} title="test" navCallback={testClick} />);
    const button = screen.getAllByTestId('hidden-commits-button')[0];
    fireEvent.click(button);
    expect(testClick).toBeCalled();
});
