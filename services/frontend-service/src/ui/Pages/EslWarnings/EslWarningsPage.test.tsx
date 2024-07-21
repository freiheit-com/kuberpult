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
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { EslWarningsPage } from './EslWarningsPage';
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';
import { FailedEslsState, updateFailedEsls } from '../../utils/store';
import { GetFailedEslsResponse } from '../../../api/api';

describe('Esl Warnings page tests', () => {
    type TestCase = {
        name: string;
        fakeLoadEverything: boolean;
        enableDex: boolean;
        enableDexValidToken: boolean;
        failedEslsStoreData:
            | {
                  failedEslsReady: FailedEslsState;
                  response: GetFailedEslsResponse | undefined;
              }
            | undefined;
        expectedSpinnerCount: number;
        expectedMainContentCount: number;
        expectedText: string;
        expectedNumLoginPage: number;
    };

    const testCases: TestCase[] = [
        {
            name: 'A loading spinner renders when the page is still loading',
            fakeLoadEverything: false,
            enableDex: false,
            enableDexValidToken: false,
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading Configuration',
            failedEslsStoreData: {
                failedEslsReady: FailedEslsState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'A spinner is shown when waiting for the server to respond',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading Failed Esls info',
            failedEslsStoreData: {
                failedEslsReady: FailedEslsState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'An error message is shown when the backend returns an error',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Backend error',
            failedEslsStoreData: {
                response: undefined,
                failedEslsReady: FailedEslsState.ERROR,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'An error message is shown when the backend returns a not found status',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'All events were processed successfully',
            failedEslsStoreData: {
                response: undefined,
                failedEslsReady: FailedEslsState.NOTFOUND,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'Some main content exists when the page is done loading',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText:
                'Failed ESL Event List: This page shows all events that could not be processed, and therefore were never written to the manifest repo. Any operation in kuberpult is an event, like creating a lock or running a release',
            failedEslsStoreData: {
                failedEslsReady: FailedEslsState.READY,
                response: {
                    failedEsls: [
                        {
                            eslId: 1,
                            createdAt: new Date('2024-02-09T11:20:00Z'),
                            eventType: 'EvtCreateApplicationVersion',
                            json: '{"version": 1, "app": "test-app-name"}',
                        },
                    ],
                },
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'A login page renders when Dex is enabled',
            fakeLoadEverything: true,
            enableDex: true,
            enableDexValidToken: false,
            expectedSpinnerCount: 0,
            expectedMainContentCount: 0,
            expectedText: 'Log in to Dex',
            failedEslsStoreData: {
                failedEslsReady: FailedEslsState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 1,
        },
        {
            name: 'Some main content exists when Dex is enabled and the token is valid',
            fakeLoadEverything: true,
            enableDex: true,
            enableDexValidToken: true,
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText:
                'Failed ESL Event List: This page shows all events that could not be processed, and therefore were never written to the manifest repo. Any operation in kuberpult is an event, like creating a lock or running a release',
            failedEslsStoreData: {
                failedEslsReady: FailedEslsState.READY,
                response: {
                    failedEsls: [
                        {
                            eslId: 1,
                            createdAt: new Date('2024-02-09T11:20:00Z'),
                            eventType: 'EvtCreateApplicationVersion',
                            json: '{"version": 1, "app": "test-app-name"}',
                        },
                    ],
                },
            },
            expectedNumLoginPage: 0,
        },
    ];
    describe.each(testCases)('', (tc) => {
        test(tc.name, () => {
            fakeLoadEverything(tc.fakeLoadEverything);
            if (tc.failedEslsStoreData !== undefined) updateFailedEsls.set(tc.failedEslsStoreData);
            if (tc.enableDex) {
                enableDexAuth(tc.enableDexValidToken);
            }

            const { container } = render(
                <MemoryRouter initialEntries={['/ui/eslWarnings/']}>
                    <Routes>
                        <Route path={'/ui/eslWarnings/'} element={<EslWarningsPage />} />
                    </Routes>
                </MemoryRouter>
            );

            expect(container.getElementsByClassName('spinner')).toHaveLength(tc.expectedSpinnerCount);
            expect(container.getElementsByClassName('main-content esl-warnings-page')).toHaveLength(
                tc.expectedMainContentCount
            );
            expect(
                container.getElementsByClassName('button-main env-card-deploy-btn mdc-button--unelevated')
            ).toHaveLength(tc.expectedNumLoginPage);
            expect(container.textContent).toContain(tc.expectedText);
        });
    });
});
