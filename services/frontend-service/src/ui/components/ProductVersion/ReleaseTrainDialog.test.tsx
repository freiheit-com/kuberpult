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

import { Spy } from 'spy4js';
import { ReleaseTrainDialog, ReleaseTrainDialogProps } from './ReleaseTrainDialog';
import { render, screen } from '@testing-library/react';
import { Environment } from '../../../api/api';
import { documentQuerySelectorSafe } from '../../../setupTests';

const myCancelSpy = jest.fn();
const mock_UseEnvs = Spy('envs');

jest.mock('../../utils/store', () => ({
    useEnvironments() {
        return mock_UseEnvs();
    },
}));

describe('Test table filtering', () => {
    type TestData = {
        name: string;
        input: ReleaseTrainDialogProps;
        envsList: Environment[];
        expectedEnvs: string[];
        filteredEnvs: string[];
    };
    const data: TestData[] = [
        {
            name: 'no environments to select',
            input: {
                environment: 'dev',
                open: true,
                onCancel: myCancelSpy,
            },
            envsList: [],
            expectedEnvs: [],
            filteredEnvs: [],
        },
        {
            name: 'environments to be displayed',
            input: {
                environment: 'dev',
                open: true,
                onCancel: myCancelSpy,
            },
            envsList: [
                {
                    name: 'test',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
            ],
            expectedEnvs: ['test'],
            filteredEnvs: [],
        },
        {
            name: 'multiple environments to be displayed',
            input: {
                environment: 'dev',
                open: true,
                onCancel: myCancelSpy,
            },
            envsList: [
                {
                    name: 'test',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
                {
                    name: 'test1',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
                {
                    name: 'test2',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
                {
                    name: 'test3',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
            ],
            expectedEnvs: ['test', 'test1', 'test2', 'test3'],
            filteredEnvs: [],
        },
        {
            name: 'multiple environments to be displayed but filter out one',
            input: {
                environment: 'dev',
                open: true,
                onCancel: myCancelSpy,
            },
            envsList: [
                {
                    name: 'test',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
                {
                    name: 'test1',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'staging' } },
                },
                {
                    name: 'test2',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
                {
                    name: 'test3',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'dev' } },
                },
            ],
            expectedEnvs: ['test', 'test2', 'test3'],
            filteredEnvs: ['test1'],
        },
        {
            name: 'filter out all environments',
            input: {
                environment: 'dev',
                open: true,
                onCancel: myCancelSpy,
            },
            envsList: [
                {
                    name: 'test',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'staging' } },
                },
                {
                    name: 'test1',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'staging' } },
                },
                {
                    name: 'test2',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'staging' } },
                },
                {
                    name: 'test3',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {},
                    config: { upstream: { environment: 'staging' } },
                },
            ],
            filteredEnvs: ['test', 'test1', 'test2', 'test3'],
            expectedEnvs: [],
        },
    ];
    describe.each(data)(`Displays Product Version Table`, (testCase) => {
        const getNode = (overrides: ReleaseTrainDialogProps) => <ReleaseTrainDialog {...overrides} />;
        const getWrapper = (overrides: ReleaseTrainDialogProps) => render(getNode(overrides));
        it(testCase.name, () => {
            mock_UseEnvs.returns(testCase.envsList);
            myCancelSpy.mockReset();
            expect(myCancelSpy).toHaveBeenCalledTimes(0);
            getWrapper(testCase.input);
            testCase.expectedEnvs.forEach((value, index) => {
                expect(documentQuerySelectorSafe('.id-' + value)).toBeDefined();
            });
            testCase.filteredEnvs.forEach((value, index) => {
                expect(screen.queryByText(value)).not.toBeInTheDocument();
            });
        });
    });
});
