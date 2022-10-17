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
import { UpdateOverview } from '../../utils/store';
import { EnvironmentLane } from './EnvironmentLane';

const sampleEnvs = {
    foo: {
        name: 'foo',
        locks: {
            testId: {
                message: 'test message',
                lockId: 'testId',
                createdBy: {
                    name: 'TestUser',
                    email: 'testuser@test.com',
                },
            },
            anotherTestId: {
                message: 'more test messages',
                lockId: 'anotherTestId',
                createdBy: {
                    name: 'TestUser',
                    email: 'testuser@test.com',
                },
            },
        },
    },
    moreTest: {
        name: 'moreTest',
        locks: {},
    },
} as any;

interface dataT {
    name: string;
    environment: string;
    expected: number;
}

const cases: dataT[] = [
    {
        name: 'Environment row with two locks',
        environment: 'foo',
        expected: 2,
    },
    {
        name: 'Environment row with no locks',
        environment: 'moreTest',
        expected: 0,
    },
    {
        name: 'None existant environment',
        environment: 'nonExistant',
        expected: 0,
    },
];

describe('Environment Lane', () => {
    const getNode = (overrides: { environment: string }) => <EnvironmentLane {...overrides} />;
    const getWrapper = (overrides: { environment: string }) => render(getNode(overrides));

    describe.each(cases)('Renders a row of environments', (testcase) => {
        it(testcase.name, () => {
            //given
            UpdateOverview.set({
                environments: sampleEnvs,
            });
            // when
            const { container } = getWrapper({ environment: testcase.environment });
            // then
            expect(container.getElementsByClassName('environment-lock-display')).toHaveLength(testcase.expected);
        });
    });
});
