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
import { updateAppDetails, UpdateOverview } from '../../utils/store';
import {
    Environment,
    EnvironmentGroup,
    Environment_Application,
    Lock,
    Priority,
    Release,
    UndeploySummary,
    GetAppDetailsResponse,
} from '../../../api/api';
import { MemoryRouter } from 'react-router-dom';

describe('Release Dialog', () => {
    type TestData = {
        name: string;
        dates: number;
        releases: Release[];
        envGroups: EnvironmentGroup[];
        expectedAppLocksLength: number;
        appDetails: { [p: string]: GetAppDetailsResponse };
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

    const app1Details: GetAppDetailsResponse = {
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
    };

    const testApp1: Environment_Application = {
        name: 'test',
        version: 1,
        locks: { testlockId: testAppLock },
        queuedVersion: 0,
        undeployVersion: false,
        teamLocks: {},
        team: 'test-team',
    };
    const testApp2: Environment_Application = {
        name: 'test2',
        version: 1,
        locks: { testlockId2: testAppLock2 },
        queuedVersion: 0,
        undeployVersion: false,
        teamLocks: {},
        team: 'test-team',
    };

    const testApp3: Environment_Application = {
        name: 'test',
        version: 2,
        locks: { testlockId3: testApplock3 },
        queuedVersion: 0,
        undeployVersion: false,
        teamLocks: {},
        team: 'test-team',
    };
    const testEnv1: Environment = {
        name: 'dev',
        applications: { test: testApp1, test2: testApp2 },
        locks: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    };
    const testEnv2: Environment = {
        name: 'staging',
        applications: { test: testApp3 },
        locks: {},
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
                },
            ],
            envGroups: [testEnvGroup1],
            expectedAppLocksLength: 1,
        },
        {
            name: '3 releases in 2 days',
            dates: 2,
            appDetails: {
                test: {
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
                },
            ],
            envGroups: [],
            expectedAppLocksLength: 0,
        },
        {
            name: 'two application locks without any release',
            dates: 0,
            appDetails: {
                test: {
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
                applications: {},
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
