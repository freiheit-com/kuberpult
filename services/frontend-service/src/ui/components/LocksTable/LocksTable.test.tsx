/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { render } from '@testing-library/react';
import React from 'react';
import { AppLockDisplay } from '../LockDisplay/AppLockDisplay';
import { EnvLockDisplay } from '../LockDisplay/EnvLockDisplay';
import { UpdateOverview } from '../../utils/store';
import { Lock } from '../../../api/api';

describe('Run Locks Table', () => {
    interface envDataT {
        name: string;
        locks: { [key: string]: Lock };
        lockId: string;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getEnvNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <EnvLockDisplay {...defaultProps} {...overrides} />;
    };
    const getEnvWrapper = (overrides: { lockID: string }) => render(getEnvNode(overrides));

    const sampleEnvData: envDataT[] = [
        {
            name: 'one normal Environment lock',
            locks: { testLock: { lockId: 'test-id', message: 'test-message', createdAt: new Date() } },
            lockId: 'test-id',
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one outdated Environment lock',
            locks: {
                testLock: { lockId: 'test-id', message: 'test-message', createdAt: new Date('1995-12-17T03:24:00') },
            },
            lockId: 'test-id',
            expect: (container) => expect(container.getElementsByClassName('date-display--outdated')).toHaveLength(1),
        },
    ];

    describe.each(sampleEnvData)(`Renders an Environment Lock Display`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                environments: {
                    integration: {
                        name: 'integration',
                        applications: {},
                        locks: testcase.locks,
                        distanceToUpstream: 0,
                        priority: 0,
                    },
                },
            });
            // when
            const { container } = getEnvWrapper({ lockID: testcase.lockId });

            testcase.expect(container);
        });
    });
    interface appDataT {
        name: string;
        locks: { [key: string]: Lock };
        lockId: string;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getAppNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <AppLockDisplay {...defaultProps} {...overrides} />;
    };
    const getAppWrapper = (overrides: { lockID: string }) => render(getAppNode(overrides));

    const sampleAppData: appDataT[] = [
        {
            name: 'one normal Application lock',
            locks: { testLock: { lockId: 'test-id', message: 'test-message', createdAt: new Date() } },
            lockId: 'test-id',
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one outdated Application lock',
            locks: {
                testLock: { lockId: 'test-id', message: 'test-message', createdAt: new Date('1995-12-17T03:24:00') },
            },
            lockId: 'test-id',
            expect: (container) => expect(container.getElementsByClassName('date-display--outdated')).toHaveLength(1),
        },
    ];

    describe.each(sampleAppData)(`Renders an Application Lock Display`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                environments: {
                    integration: {
                        name: 'integration',
                        applications: {
                            testApp: {
                                name: 'testApp',
                                locks: testcase.locks,
                                queuedVersion: 0,
                                undeployVersion: false,
                                version: 0,
                            },
                        },
                        locks: {},
                        distanceToUpstream: 0,
                        priority: 0,
                    },
                },
            });
            // when
            const { container } = getAppWrapper({ lockID: testcase.lockId });
            testcase.expect(container);
        });
    });
});
