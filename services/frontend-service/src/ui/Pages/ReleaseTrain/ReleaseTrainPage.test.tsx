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
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';
import { ReleaseTrainPrognosisState, updateReleaseTrainPrognosis } from '../../utils/store';
import { GetReleaseTrainPrognosisResponse, ReleaseTrainEnvSkipCause } from '../../../api/api';
import { ReleaseTrainPage } from './ReleaseTrainPage';

describe('Commit info page tests', () => {
    type TestCase = {
        name: string;
        envName: string;
        fakeLoadEverything: boolean;
        enableDex: boolean;
        enableDexValidToken: boolean;
        releaseTrainPrognosisStoreData:
            | {
                  releaseTrainPrognosisReady: ReleaseTrainPrognosisState;
                  response: GetReleaseTrainPrognosisResponse | undefined;
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
            envName: 'development',
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading Configuration',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'An Error is shown when the environemnt name is not provided in the URL',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            envName: '',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Environment name not provided',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'A spinner is shown when waiting for the server to respond',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            envName: 'development',
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading release train prognosis...',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'An error message is shown when the backend returns an error',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            envName: 'development',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Backend error',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.ERROR,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'An error message is shown when the backend returns a not found status',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            envName: 'development',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'The provided environment name development was not found in the manifest repository.',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.NOTFOUND,
                response: undefined,
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'Some main content exists when the page is done loading',
            fakeLoadEverything: true,
            enableDex: false,
            enableDexValidToken: false,
            envName: 'development',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Prognosis for release train on environment development',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.READY,
                response: {
                    envsPrognoses: {
                        development: {
                            outcome: {
                                $case: 'skipCause',
                                skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
                            },
                        },
                    },
                },
            },
            expectedNumLoginPage: 0,
        },
        {
            name: 'A login page renders when Dex is enabled',
            fakeLoadEverything: true,
            enableDex: true,
            enableDexValidToken: false,
            envName: 'development',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Log in to Dex',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.LOADING,
                response: undefined,
            },
            expectedNumLoginPage: 1,
        },
        {
            name: 'Some main content exists when Dex is enabled with a valid token',
            fakeLoadEverything: true,
            enableDex: true,
            enableDexValidToken: true,
            envName: 'development',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Prognosis for release train on environment development',
            releaseTrainPrognosisStoreData: {
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.READY,
                response: {
                    envsPrognoses: {
                        development: {
                            outcome: {
                                $case: 'skipCause',
                                skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
                            },
                        },
                    },
                },
            },
            expectedNumLoginPage: 0,
        },
    ];
    describe.each(testCases)('', (tc) => {
        test(tc.name, () => {
            fakeLoadEverything(tc.fakeLoadEverything);
            if (tc.releaseTrainPrognosisStoreData !== undefined)
                updateReleaseTrainPrognosis.set(tc.releaseTrainPrognosisStoreData);
            if (tc.enableDex == true) {
                enableDexAuth(tc.enableDexValidToken);
            }

            const { container } = render(
                <MemoryRouter initialEntries={['/ui/environments/' + tc.envName + '/releaseTrain']}>
                    <Routes>
                        <Route
                            path={'/ui/environments/' + (tc.envName !== '' ? ':targetEnv' : '') + '/releaseTrain'}
                            element={<ReleaseTrainPage />}
                        />
                    </Routes>
                </MemoryRouter>
            );
            expect(container.getElementsByClassName('spinner')).toHaveLength(tc.expectedSpinnerCount);
            expect(container.getElementsByClassName('main-content')).toHaveLength(tc.expectedMainContentCount);

            expect(container.textContent).toContain(tc.expectedText);
            expect(container.getElementsByClassName('button-main env-card-deploy-btn mdc-button--unelevated')).toHaveLength(tc.expectedNumLoginPage);
        });
    });
});
