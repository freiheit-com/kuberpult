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
import { EnvironmentCard } from '../../components/EnvironmentCard/EnvironmentCard';

// eslint-disable-next-line no-type-assertion/no-type-assertion
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
    const getNode = (overrides: { environment: string }) => <EnvironmentCard {...overrides} />;
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
