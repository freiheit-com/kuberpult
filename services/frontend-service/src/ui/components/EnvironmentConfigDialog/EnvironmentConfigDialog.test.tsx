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
import { act, fireEvent, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import {
    EnvironmentConfigDialog,
    environmentConfigDialogClass,
    environmentConfigDialogCloseClass,
    environmentConfigDialogConfigClass,
} from './EnvironmentConfigDialog';

jest.mock('../../../api/api').mock('../../utils/GrpcApi');

const configContent = 'config content';
const configLoadError = new Error('error loading config');
const mockGetEnvironmentConfigPretty = jest.fn();
const mockShowSnackbarError = jest.fn();
jest.mock('../../utils/store', () => ({
    GetEnvironmentConfigPretty: () => mockGetEnvironmentConfigPretty(),
    showSnackbarError: () => mockShowSnackbarError(),
}));

const openEnvironmentDialog = 'open environment';
const closedEnvironmentDialog = 'closed environment';
const mockGetOpenEnvironmentConfigDialog = jest.fn();
const mockSetOpenEnvironmentConfigDialog = jest.fn();
jest.mock('../../utils/Links', () => ({
    getOpenEnvironmentConfigDialog: () => mockGetOpenEnvironmentConfigDialog(),
    setOpenEnvironmentConfigDialog: () => mockSetOpenEnvironmentConfigDialog(),
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
        expectedNumCloseButtons: number;
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
            expectedNumCloseButtons: 0,
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
            expectedNumCloseButtons: 1,
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
            expectedNumCloseButtons: 1,
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
            expectedNumCloseButtons: 1,
        },
    ];

    it.each(data)(`Renders and tests an environment config dialog: %s`, async (testcase) => {
        // when
        mockGetEnvironmentConfigPretty.mockImplementation(testcase.config);
        mockGetOpenEnvironmentConfigDialog.mockImplementation(() => openEnvironmentDialog);
        const element = getNode(testcase.environmentName);
        const container = render(element).container;
        await act(global.nextTick);

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
        expect(mockShowSnackbarError).toHaveBeenCalledTimes(testcase.expectedSnackbarErrorCalls);

        const closers = container.getElementsByClassName(environmentConfigDialogCloseClass);
        expect(closers).toHaveLength(testcase.expectedNumCloseButtons);
        expect(mockSetOpenEnvironmentConfigDialog).toHaveBeenCalledTimes(0);
        await act(async () => {
            for (const closer of closers) {
                fireEvent.click(closer);
            }
        });
        expect(mockSetOpenEnvironmentConfigDialog).toHaveBeenCalledTimes(testcase.expectedNumCloseButtons);
    });
});
