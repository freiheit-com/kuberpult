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
import { Releases } from './Releases';
import { render } from '@testing-library/react';
import { AppDetailsResponse, AppDetailsState, updateAppDetails, UpdateOverview } from '../../utils/store';
import {
    Environment,
    EnvironmentGroup,
    Lock,
    OverviewApplication,
    Priority,
    Release,
    UndeploySummary,
} from '../../../api/api';
import { MemoryRouter } from 'react-router-dom';

describe('Release Dialog', () => {
    type TestData = {
        name: string;
        dates: number;
        releases: Release[];
        OverviewApps: OverviewApplication[];
        envGroups: EnvironmentGroup[];
        expectedAppLocksLength: number;
        appDetails: { [p: string]: AppDetailsResponse };
    };

    const releases = [
        {
            version: 1,
            sourceMessage: 'test1',
            sourceAuthor: 'test',
            sourceCommitId: 'commit',
            createdAt: new Date('2022-12-04T12:30:12'),
            undeployVersion: false,
            prNumber: '666',
            displayVersion: '1',
            isMinor: false,
            isPrepublish: false,
            environments: [],
        },
        {
            version: 2,
            sourceMessage: 'test1',
            sourceAuthor: 'test',
            sourceCommitId: 'commit',
            createdAt: new Date('2022-12-05T12:30:12'),
            undeployVersion: false,
            prNumber: '666',
            displayVersion: '2',
            isMinor: false,
            isPrepublish: false,
            environments: [],
        },
        {
            version: 3,
            sourceMessage: 'test1',
            sourceAuthor: 'test',
            sourceCommitId: 'commit',
            createdAt: new Date('2022-12-06T12:30:12'),
            undeployVersion: false,
            prNumber: '666',
            displayVersion: '3',
            isMinor: false,
            isPrepublish: false,
            environments: [],
        },
    ];

    const testAppLock: Lock = {
        lockId: 'testlockId',
        message: 'test-lock',
        createdAt: new Date('2022-12-04T12:30:12'),
        createdBy: { name: 'test', email: 'test' },
    };
    const testAppLock2: Lock = {
        lockId: 'testlockId2',
        message: 'test-lock',
        createdAt: new Date('2022-12-04T12:30:12'),
        createdBy: { name: 'test', email: 'test' },
    };
    const testApplock3: Lock = {
        lockId: 'testlockId3',
        message: 'test-lock',
        createdAt: new Date('2022-12-04T12:30:12'),
        createdBy: { name: 'test', email: 'test' },
    };

    const app1Details: AppDetailsResponse = {
        details: {
            application: {
                name: 'test',
                releases: releases,
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
                undeploySummary: UndeploySummary.NORMAL,
                warnings: [],
            },
            appLocks: {
                dev: {
                    locks: [testAppLock],
                },
            },
            teamLocks: {},
            deployments: {
                dev: {
                    version: 1,
                    queuedVersion: 0,
                    undeployVersion: false,
                },
            },
        },
        appDetailState: AppDetailsState.READY,
        updatedAt: new Date(Date.now()),
    };

    const testEnv1: Environment = {
        name: 'dev',
        locks: {},
        appLocks: {
            test: {
                locks: [testAppLock],
            },
            test2: {
                locks: [testAppLock2],
            },
        },
        teamLocks: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    };
    const testEnv2: Environment = {
        name: 'staging',
        locks: {},
        appLocks: {
            test: {
                locks: [testApplock3],
            },
        },
        teamLocks: {},
        distanceToUpstream: 0,
        priority: Priority.PROD,
    };
    const testEnvGroup1: EnvironmentGroup = {
        environmentGroupName: 'development',
        environments: [testEnv1],
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    };
    const testEnvGroup2: EnvironmentGroup = {
        environmentGroupName: 'staging',
        environments: [testEnv2],
        distanceToUpstream: 0,
        priority: Priority.PROD,
    };

    const data: TestData[] = [
        {
            appDetails: {
                test: app1Details,
            },
            OverviewApps: [
                {
                    name: app1Details.details?.application?.name || '',
                    team: app1Details.details?.application?.team || '',
                },
            ],
            name: '3 releases in 3 days',
            dates: 3,
            releases: [
                {
                    version: 1,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '1',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-05T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '3',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
            ],
            envGroups: [testEnvGroup1],
            expectedAppLocksLength: 1,
        },
        {
            name: '3 releases in 2 days',
            dates: 2,
            OverviewApps: [
                {
                    name: 'test',
                    team: 'example',
                },
            ],
            appDetails: {
                test: {
                    details: {
                        application: {
                            name: 'test',
                            releases: [
                                {
                                    version: 1,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date('2022-12-04T12:30:12'),
                                    undeployVersion: false,
                                    prNumber: '666',
                                    displayVersion: '1',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 2,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date('2022-12-04T15:30:12'),
                                    undeployVersion: false,
                                    prNumber: '666',
                                    displayVersion: '2',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                                {
                                    version: 3,
                                    sourceMessage: 'test1',
                                    sourceAuthor: 'test',
                                    sourceCommitId: 'commit',
                                    createdAt: new Date('2022-12-06T12:30:12'),
                                    undeployVersion: false,
                                    prNumber: '666',
                                    displayVersion: '3',
                                    isMinor: false,
                                    isPrepublish: false,
                                    environments: [],
                                },
                            ],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {},
                        teamLocks: {},
                        deployments: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                },
            },
            releases: [
                {
                    version: 1,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '1',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T15:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '2',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '3',
                    isMinor: false,
                    isPrepublish: false,
                    environments: [],
                },
            ],
            envGroups: [],
            expectedAppLocksLength: 0,
        },
        {
            name: 'two application locks without any release',
            dates: 0,
            OverviewApps: [
                {
                    name: 'test',
                    team: 'example',
                },
            ],
            appDetails: {
                test: {
                    details: {
                        application: {
                            name: 'test',
                            releases: [],
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        appLocks: {},
                        teamLocks: {},
                        deployments: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                },
            },
            releases: [],
            envGroups: [testEnvGroup1, testEnvGroup2],
            expectedAppLocksLength: 2,
        },
    ];

    describe.each(data)(`Renders releases for an app`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                lightweightApps: testcase.OverviewApps,
                environmentGroups: testcase.envGroups,
            });
            updateAppDetails.set(testcase.appDetails);
            render(
                <MemoryRouter>
                    <Releases app="test" />
                </MemoryRouter>
            );

            expect(document.querySelectorAll('.release_date')).toHaveLength(testcase.dates);
            expect(document.querySelectorAll('.content')).toHaveLength(testcase.releases.length);
            expect(document.querySelectorAll('.application-lock-chip')).toHaveLength(testcase.expectedAppLocksLength);
        });
    });
});
