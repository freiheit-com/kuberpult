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
import { UpdateOverview } from '../../utils/store';
import { Environment, EnvironmentGroup, Priority } from '../../../api/api';
import React from 'react';
import { EnvironmentsPage } from './EnvironmentsPage';

const sampleEnvsA: Environment[] = [
    {
        name: 'foo',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
    {
        name: 'moreTest',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
];

const sampleEnvsB: Environment[] = [
    {
        name: 'fooB',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
    {
        name: 'moreTestB',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
];

describe('Environment Lane', () => {
    const getNode = () => <EnvironmentsPage />;
    const getWrapper = () => render(getNode());

    interface dataT {
        name: string;
        environmentGroups: EnvironmentGroup[];
        loaded: boolean;
        expected: number;
        expectedEnvHeaderWrapper: number;
        expectedMainContent: number;
        spinnerExpected: number;
    }
    const cases: dataT[] = [
        {
            name: '1 group 1 env',
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
            ],
            loaded: true,
            expected: 1,
            expectedEnvHeaderWrapper: 1,
            expectedMainContent: 1,
            spinnerExpected: 0,
        },
        {
            name: '2 group 1 env each',
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
                {
                    environments: [sampleEnvsB[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
            ],
            loaded: true,
            expected: 1,
            expectedEnvHeaderWrapper: 2,
            expectedMainContent: 1,
            spinnerExpected: 0,
        },
        {
            name: '1 group 2 env',
            environmentGroups: [
                {
                    environments: sampleEnvsA,
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
                {
                    environments: sampleEnvsB,
                    distanceToUpstream: 1,
                    environmentGroupName: 'g2',
                },
            ],
            loaded: true,
            expected: 2,
            expectedEnvHeaderWrapper: 4,
            expectedMainContent: 1,
            spinnerExpected: 0,
        },
        {
            name: 'just the spinner',
            environmentGroups: [],
            loaded: false,
            expected: 0,
            expectedEnvHeaderWrapper: 0,
            expectedMainContent: 0,
            spinnerExpected: 1,
        },
    ];
    describe.each(cases)('Renders a row of environments', (testcase) => {
        it(testcase.name, () => {
            //given
            UpdateOverview.set({
                environmentGroups: testcase.environmentGroups,
                loaded: testcase.loaded,
            });
            // when
            const { container } = getWrapper();
            // then
            expect(container.getElementsByClassName('spinner')).toHaveLength(testcase.spinnerExpected);
            expect(container.getElementsByClassName('environment-group-lane')).toHaveLength(testcase.expected);
            expect(container.getElementsByClassName('main-content')).toHaveLength(testcase.expectedMainContent);
            expect(container.getElementsByClassName('environment-lane__header')).toHaveLength(
                testcase.expectedEnvHeaderWrapper
            );
        });
    });
});
