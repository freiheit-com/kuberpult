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

import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { EnvironmentGroup, Priority } from '../../../api/api';
import { EnvironmentGroupCard } from './EnvironmentCard';
import { UpdateOverview } from '../../utils/store';

const getNode = (group: EnvironmentGroup): JSX.Element | any => (
    <MemoryRouter>
        <EnvironmentGroupCard environmentGroup={group} />
    </MemoryRouter>
);
const getWrapper = (group: EnvironmentGroup) => render(getNode(group));

describe('Test Environment Cards', () => {
    interface dataEnvT {
        name: string;
        group: EnvironmentGroup;
        expectedNumEnvLockButtons: number;
        expectedNumGroupsLockButtons: number;
        expectedPriorityClassName: string;
        expectedNumButtonsEnv: number;
        expectedNumEnvLinks: number;
    }

    const sampleEnvData: dataEnvT[] = [
        {
            name: '1 group 0 envs',
            group: {
                environmentGroupName: 'group1',
                distanceToUpstream: 2,
                priority: Priority.UNRECOGNIZED,
                environments: [],
            },
            expectedNumGroupsLockButtons: 1,
            expectedNumEnvLockButtons: 0,
            expectedPriorityClassName: 'environment-priority-unrecognized', // group priority is UNRECOGNIZED / unknown
            expectedNumButtonsEnv: 1,
            expectedNumEnvLinks: 0,
        },
        {
            name: '1 group 1 env',
            group: {
                environmentGroupName: 'group1',
                distanceToUpstream: 2,
                priority: Priority.PRE_PROD,
                environments: [
                    {
                        name: 'env1',
                        distanceToUpstream: 2,
                        locks: {},
                        appLocks: {},
                        teamLocks: {},
                        priority: Priority.PRE_PROD,
                        config: {},
                    },
                ],
            },
            expectedNumGroupsLockButtons: 1,
            expectedNumEnvLockButtons: 2,
            expectedPriorityClassName: 'environment-priority-pre_prod',
            expectedNumButtonsEnv: 4,
            expectedNumEnvLinks: 2,
        },
        {
            name: '1 group 2 env',
            group: {
                environmentGroupName: 'group1',
                distanceToUpstream: 2,
                priority: Priority.UPSTREAM,
                environments: [
                    {
                        name: 'env1',
                        distanceToUpstream: 2,
                        locks: {},
                        appLocks: {},
                        teamLocks: {},
                        priority: Priority.UPSTREAM,
                        config: {},
                    },
                    {
                        name: 'env2',
                        distanceToUpstream: 2,
                        locks: {},
                        appLocks: {},
                        teamLocks: {},
                        priority: Priority.UPSTREAM,
                        config: {},
                    },
                ],
            },
            expectedNumGroupsLockButtons: 1,
            expectedNumEnvLockButtons: 4,
            expectedPriorityClassName: 'environment-priority-upstream',
            expectedNumButtonsEnv: 7,
            expectedNumEnvLinks: 4,
        },
    ];

    describe.each(sampleEnvData)(`Test Lock IDs`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({
                environmentGroups: [testcase.group],
            });
            // when
            const { container } = getWrapper(testcase.group);
            // then
            const lockGroupElems = container.getElementsByClassName('test-lock-group');
            expect(lockGroupElems).toHaveLength(testcase.expectedNumGroupsLockButtons);
            const lockEnvElems = container.getElementsByClassName('test-lock-env');
            expect(lockEnvElems).toHaveLength(testcase.expectedNumEnvLockButtons);
            const buttons = container.getElementsByClassName('environment-action');
            expect(buttons).toHaveLength(testcase.expectedNumButtonsEnv);
            const envLinks = container.getElementsByClassName('environment-link');
            expect(envLinks).toHaveLength(testcase.expectedNumEnvLinks);

            // when
            const envGroupHeader = container.querySelector('.environment-group-lane__header');
            // then
            expect(envGroupHeader?.className).toContain(testcase.expectedPriorityClassName);
        });
    });
});
