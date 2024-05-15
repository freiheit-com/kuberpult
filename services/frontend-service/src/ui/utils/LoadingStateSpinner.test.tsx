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
import React from 'react';
import { LoadingStateSpinner } from './LoadingStateSpinner';
import { GlobalLoadingState } from './store';
import { elementQuerySelectorSafe } from '../../setupTests';

const cases: { name: string; inputState: GlobalLoadingState; expectedMessage: string }[] = [
    {
        name: 'Everything ready',
        inputState: {
            configReady: true,
            azureAuthEnabled: true,
            isAuthenticated: true,
            overviewLoaded: true,
        },
        expectedMessage: 'Loading...',
    },
    {
        name: 'Config not ready',
        inputState: {
            configReady: false,
            azureAuthEnabled: true,
            isAuthenticated: true,
            overviewLoaded: true,
        },
        expectedMessage: 'Loading Configuration...',
    },
    {
        name: 'Auth not ready',
        inputState: {
            configReady: true,
            azureAuthEnabled: true,
            isAuthenticated: false,
            overviewLoaded: true,
        },
        expectedMessage: 'Authenticating with Azure...',
    },
    {
        name: 'Auth not ready but also not enabled',
        inputState: {
            configReady: true,
            azureAuthEnabled: false,
            isAuthenticated: false,
            overviewLoaded: true,
        },
        expectedMessage: 'Loading...',
    },
    {
        name: 'Auth not ready',
        inputState: {
            configReady: true,
            azureAuthEnabled: true,
            isAuthenticated: true,
            overviewLoaded: false,
        },
        expectedMessage: 'Loading Overview...',
    },
];

describe('LoadingStateSpinner', () => {
    const getWrapper = (loadingState: GlobalLoadingState) =>
        render(<LoadingStateSpinner loadingState={loadingState} />);

    describe.each(cases)('Renders the correct message', (testcase) => {
        it(testcase.name, () => {
            //given
            const { container } = getWrapper(testcase.inputState);
            // when
            const message = elementQuerySelectorSafe(container, '.spinner-message');

            // then
            expect(message?.textContent).toEqual(testcase.expectedMessage);
        });
    });
});
