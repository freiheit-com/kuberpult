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
import { act, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import {
    EnvironmentConfigDialog,
    environmentConfigDialogClass,
    environmentConfigDialogConfigClass,
} from './EnvironmentConfigDialog';

jest.mock('../../../api/api').mock('../../utils/GrpcApi');

const configContent = 'config content';
const configLoadError = 'fake loading error'; //new Error('error loading config');
const mockGetEnvironmentConfigPretty = jest.fn();
const mockShowSnackbarError = jest.fn();
jest.mock('../../utils/store', () => ({
    GetEnvironmentConfigPretty: () => mockGetEnvironmentConfigPretty(),
    showSnackbarError: () => mockShowSnackbarError(),
}));

const openEnvironmentDialog = 'open environment';
const closedEnvironmentDialog = 'closed environment';
jest.mock('../../utils/Links', () => ({
    getOpenEnvironmentConfigDialog() {
        return openEnvironmentDialog;
    },
}));

describe('EnvironmentConfigDialog', () => {
    const getNode = (environmentName: string) => (
        <MemoryRouter>
            <EnvironmentConfigDialog environmentName={environmentName} />
        </MemoryRouter>
    );

    type TestData = {
        name: string;
        environmentName: string;
        config: () => Promise<string>;
        expectedNumDialogs: number;
        expectedNumConfigs: number;
        expectedConfig: string;
        expectedNumSpinners: number;
        expectedSnackbarErrorCalls: number;
    };

    const data: TestData[] = [
        {
            name: 'closed environment config',
            environmentName: closedEnvironmentDialog,
            config: () => Promise.resolve(configContent),
            expectedNumDialogs: 0,
            expectedNumConfigs: 0,
            expectedConfig: configContent,
            expectedNumSpinners: 0,
            expectedSnackbarErrorCalls: 0,
        },
        {
            name: 'open environment config',
            environmentName: openEnvironmentDialog,
            config: () => Promise.resolve(configContent),
            expectedNumDialogs: 1,
            expectedNumConfigs: 1,
            expectedConfig: configContent,
            expectedNumSpinners: 0,
            expectedSnackbarErrorCalls: 0,
        },
        {
            name: 'loading environment config',
            environmentName: openEnvironmentDialog,
            config: () => new Promise(() => {}), // never resolves
            expectedNumDialogs: 1,
            expectedNumConfigs: 0,
            expectedConfig: configContent,
            expectedNumSpinners: 1,
            expectedSnackbarErrorCalls: 0,
        },
        {
            name: 'failed loading environment config',
            environmentName: openEnvironmentDialog,
            config: () => Promise.reject(configLoadError),
            expectedNumDialogs: 1,
            expectedNumConfigs: 1,
            expectedConfig: '',
            expectedNumSpinners: 0,
            expectedSnackbarErrorCalls: 1,
        },
    ];

    describe.each(data)(`Renders an environment config dialog`, (testcase) => {
        it(testcase.name, async () => {
            // when
            mockGetEnvironmentConfigPretty.mockImplementation(testcase.config);
            const element = getNode(testcase.environmentName);
            var container = render(<></>).container;
            await act(async () => {
                container = render(element).container;
            });

            // then
            expect(container.getElementsByClassName(environmentConfigDialogClass)).toHaveLength(
                testcase.expectedNumDialogs
            );
            const configs = container.getElementsByClassName(environmentConfigDialogConfigClass);
            expect(configs).toHaveLength(testcase.expectedNumConfigs);
            for (const config of configs) {
                expect(config.innerHTML).toContain(testcase.expectedConfig);
            }
            const spinners = container.getElementsByClassName('spinner-message');
            expect(spinners).toHaveLength(testcase.expectedNumSpinners);
            expect(mockShowSnackbarError.mock.calls.length).toEqual(testcase.expectedSnackbarErrorCalls);
        });
    });
});
