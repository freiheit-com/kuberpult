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
import { UpdateOverview } from '../../utils/store';
import { Environment, EnvironmentGroup, Priority } from '../../../api/api';
import React from 'react';
import { EnvironmentsPage } from './EnvironmentsPage';
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';
import { MemoryRouter } from 'react-router-dom';

const sampleEnvsA: Environment[] = [
    {
        name: 'foo',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.YOLO,
    },
    {
        name: 'moreTest',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.YOLO,
    },
];

const sampleEnvsB: Environment[] = [
    {
        name: 'fooB',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.YOLO,
    },
    {
        name: 'moreTestB',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.YOLO,
    },
];

describe('Environment Lane', () => {
    const getNode = () => (
        <MemoryRouter>
            <EnvironmentsPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    interface dataT {
        name: string;
        environmentGroups: EnvironmentGroup[];
        loaded: boolean;
        enableDex: boolean;
        enableDexValidToken: boolean,
        expected: number;
        expectedEnvHeaderWrapper: number;
        expectedMainContent: number;
        spinnerExpected: number;
        expectedCardStyles: { className: string; count: number }[];
        expectedNumLoginPage: number, 
    }
    const cases: dataT[] = [
        {
            name: '1 group 1 env',
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.YOLO,
                },
            ],
            loaded: true,
            enableDex: false, 
            enableDexValidToken: false, 
            expected: 1,
            expectedEnvHeaderWrapper: 1,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [
                {
                    className: 'environment-priority-yolo',
                    count: 1,
                },
            ],
            expectedNumLoginPage: 0, 
        },
        {
            name: '2 group 1 env each',
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.YOLO,
                },
                {
                    environments: [sampleEnvsB[0]],
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.YOLO,
                },
            ],
            loaded: true,
            enableDex: false, 
            enableDexValidToken: false, 
            expected: 1,
            expectedEnvHeaderWrapper: 2,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [
                {
                    className: 'environment-priority-yolo',
                    count: 2,
                },
            ],
            expectedNumLoginPage: 0, 
        },
        {
            name: '1 group 2 env',
            environmentGroups: [
                {
                    environments: sampleEnvsA,
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.YOLO,
                },
            ],
            loaded: true,
            enableDex: false, 
            enableDexValidToken: false, 
            expected: 1,
            expectedEnvHeaderWrapper: 2,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [
                {
                    className: 'environment-priority-yolo',
                    count: 3,
                },
            ],
            expectedNumLoginPage: 0, 
        },
        {
            name: 'card colors are decided by group priority not environment priority',
            environmentGroups: [
                {
                    environments: sampleEnvsA,
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.UPSTREAM,
                },
            ],
            loaded: true,
            enableDex: false, 
            enableDexValidToken: false, 
            expected: 1,
            expectedEnvHeaderWrapper: 2,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [
                {
                    className: 'environment-priority-yolo',
                    count: 0,
                },
                {
                    className: 'environment-priority-upstream',
                    count: 3,
                },
            ],
            expectedNumLoginPage: 0, 
        },
        {
            name: 'just the spinner',
            environmentGroups: [],
            loaded: false,
            enableDex: false, 
            enableDexValidToken: false, 
            expected: 0,
            expectedEnvHeaderWrapper: 0,
            expectedMainContent: 0,
            spinnerExpected: 1,
            expectedCardStyles: [],
            expectedNumLoginPage: 0,
        },
        {
            name: 'A login page renders when Dex is enabled',
            environmentGroups: [],
            loaded: true,
            enableDex: true, 
            enableDexValidToken: false, 
            expected: 0,
            expectedEnvHeaderWrapper: 0,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [],
            expectedNumLoginPage: 1,
        },
        {
            name: '1 group 1 env with Dex enabled and a valid token',
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 0,
                    environmentGroupName: 'g1',
                    priority: Priority.YOLO,
                },
            ],
            loaded: true,
            enableDex: true, 
            enableDexValidToken: true, 
            expected: 1,
            expectedEnvHeaderWrapper: 1,
            expectedMainContent: 1,
            spinnerExpected: 0,
            expectedCardStyles: [
                {
                    className: 'environment-priority-yolo',
                    count: 1,
                },
            ],
            expectedNumLoginPage: 0, 
        },
    ];
    describe.each(cases)('Renders a row of environments', (testcase) => {
        it(testcase.name, () => {
            //given
            UpdateOverview.set({
                environmentGroups: testcase.environmentGroups,
            });
            fakeLoadEverything(testcase.loaded);
            if (testcase.enableDex == true) {
                enableDexAuth(testcase.enableDexValidToken)
            }
            // when
            const { container } = getWrapper();
            // then
            expect(container.getElementsByClassName('spinner')).toHaveLength(testcase.spinnerExpected);
            expect(container.getElementsByClassName('environment-group-lane')).toHaveLength(testcase.expected);
            expect(container.getElementsByClassName('main-content')).toHaveLength(testcase.expectedMainContent);
            expect(container.getElementsByClassName('login-page')).toHaveLength(testcase.expectedNumLoginPage);
            expect(container.getElementsByClassName('environment-lane__header')).toHaveLength(
                testcase.expectedEnvHeaderWrapper
            );
            for (const expectedCardStyle of testcase.expectedCardStyles) {
                expect(container.getElementsByClassName(expectedCardStyle.className)).toHaveLength(
                    expectedCardStyle.count
                );
            }
        });
    });
});
