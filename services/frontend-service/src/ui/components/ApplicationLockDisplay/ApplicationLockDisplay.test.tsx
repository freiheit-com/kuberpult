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

import { fireEvent, render } from '@testing-library/react';
import { Environment, EnvironmentGroup, Environment_Application, Lock, Priority } from '../../../api/api';
import { ApplicationLockChip } from './ApplicationLockDisplay';
import { DisplayApplicationLock } from '../../utils/store';
import { Spy } from 'spy4js';

const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

describe('ApplicationLockDisplay', () => {
    type TestCase = {
        name: string;
        displayLock: DisplayApplicationLock;
        expectedPriorityClassName: string;
        expectedTitle: string;
    };
    const testAppLock: Lock = {
        lockId: 'testlockId',
        message: 'test-lock',
        createdAt: new Date('2022-12-04T12:30:12'),
        createdBy: { name: 'test', email: 'test' },
    };
    const testApp: Environment_Application = {
        name: 'test',
        version: 1,
        locks: { testlockId: testAppLock },
        queuedVersion: 0,
        undeployVersion: false,
        teamLocks: {},
        team: 'test-team',
    };
    const testEnv1: Environment = {
        name: 'dev',
        applications: { test: testApp },
        locks: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
        appLocks: {},
        teamLocks: {},
    };
    const testEnv2: Environment = {
        name: 'staging',
        applications: { test: testApp },
        locks: {},
        distanceToUpstream: 0,
        priority: Priority.OTHER,
        appLocks: {},
        teamLocks: {},
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
        priority: Priority.OTHER,
    };
    const testCases: TestCase[] = [
        {
            name: 'test lock with upstream priority',
            displayLock: {
                environment: testEnv1,
                environmentGroup: testEnvGroup1,
                application: testApp,
                lock: {
                    date: testAppLock.createdAt,
                    environment: testEnv1.name,
                    application: testApp.name,
                    team: testApp.team,
                    message: testAppLock.message,
                    lockId: testAppLock.lockId,
                    authorName: testAppLock.createdBy?.name,
                    authorEmail: testAppLock.createdBy?.email,
                },
            },
            expectedPriorityClassName: '.environment-priority-upstream',
            expectedTitle: 'dev',
        },
        {
            name: 'test lock with different priority',
            displayLock: {
                environment: testEnv2,
                environmentGroup: testEnvGroup2,
                application: testApp,
                lock: {
                    date: testAppLock.createdAt,
                    environment: testEnv2.name,
                    application: testApp.name,
                    team: testApp.team,
                    message: testAppLock.message,
                    lockId: testAppLock.lockId,
                    authorName: testAppLock.createdBy?.name,
                    authorEmail: testAppLock.createdBy?.email,
                },
            },
            expectedPriorityClassName: '.environment-priority-other',
            expectedTitle: 'staging',
        },
    ];
    describe.each(testCases)(`Renders ApplicationLockDisplay`, (testCase) => {
        it(testCase.name, () => {
            const { container } = render(
                <ApplicationLockChip
                    environment={testCase.displayLock.environment}
                    application={testCase.displayLock.application}
                    environmentGroup={testCase.displayLock.environmentGroup}
                    lock={testCase.displayLock.lock}
                />
            );
            expect(container.querySelectorAll(testCase.expectedPriorityClassName)).toHaveLength(1);
            expect(container.querySelector('.mdc-evolution-chip__text-name')?.textContent).toBe(testCase.expectedTitle);
            const lockButton = container.querySelectorAll('.button-lock')[0];
            fireEvent.click(lockButton);
            mock_addAction.addAction.wasCalled();
            expect(mock_addAction.addAction.getAllCallArguments()[0][0]).toHaveProperty(
                'action.deleteEnvironmentApplicationLock.lockId',
                testCase.displayLock.lock.lockId
            );
        });
    });
});
