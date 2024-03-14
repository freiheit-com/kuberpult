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
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { CommitInfoPage } from './CommitInfoPage';
import { fakeLoadEverything } from '../../../setupTests';
import { updateCommitInfo, CommitInfoState } from '../../utils/store';
import { GetCommitInfoResponse } from '../../../api/api';

describe('Commit info page tests', () => {
    type TestCase = {
        name: string;
        commitHash: string;
        fakeLoadEverything: boolean;
        commitInfoStoreData:
            | {
                  commitInfoReady: CommitInfoState;
                  response: GetCommitInfoResponse | undefined;
              }
            | undefined;
        expectedSpinnerCount: number;
        expectedMainContentCount: number;
        expectedText: string;
    };

    const testCases: TestCase[] = [
        {
            name: 'A loading spinner renders when the page is still loading',
            fakeLoadEverything: false,
            commitHash: 'potato',
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading Configuration',
            commitInfoStoreData: {
                commitInfoReady: CommitInfoState.LOADING,
                response: undefined,
            },
        },
        {
            name: 'An Error is shown when the commit ID is not provided in the URL',
            fakeLoadEverything: true,
            commitHash: '',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'commit ID not provided',
            commitInfoStoreData: {
                commitInfoReady: CommitInfoState.LOADING,
                response: undefined,
            },
        },
        {
            name: 'A spinner is shown when waiting for the server to respond',
            fakeLoadEverything: true,
            commitHash: 'potato',
            expectedSpinnerCount: 1,
            expectedMainContentCount: 0,
            expectedText: 'Loading commit info',
            commitInfoStoreData: {
                commitInfoReady: CommitInfoState.LOADING,
                response: undefined,
            },
        },
        {
            name: 'An error message is shown when the backend returns an error',
            fakeLoadEverything: true,
            commitHash: 'potato',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Backend error',
            commitInfoStoreData: {
                response: undefined,
                commitInfoReady: CommitInfoState.ERROR,
            },
        },
        {
            name: 'An error message is shown when the backend returns a not found status',
            fakeLoadEverything: true,
            commitHash: 'potato',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText:
                'The provided commit ID was not found in the manifest repo. This is because either the commit ID is incorrect, or it refers to an old commit whose release has been cleaned up by now.',
            commitInfoStoreData: {
                response: undefined,
                commitInfoReady: CommitInfoState.NOTFOUND,
            },
        },
        {
            name: 'Some main content exists when the page is done loading',
            fakeLoadEverything: true,
            commitHash: 'potato',
            expectedSpinnerCount: 0,
            expectedMainContentCount: 1,
            expectedText: 'Commit Add google to windows', // this "Commit + commit_message_first_line" string comes from the CommitInfo component logic (so we know that it actually rendered without having some mocking magic)
            commitInfoStoreData: {
                commitInfoReady: CommitInfoState.READY,
                response: {
                    commitHash: 'potato',
                    touchedApps: ['google', 'windows'],
                    commitMessage: `Add google to windows
Commit message body line 1
Commit message body line 2`,
                    events: [],
                    previousCommitHash: '',
                    nextCommitHash: '',
                },
            },
        },
    ];
    describe.each(testCases)('', (tc) => {
        test(tc.name, () => {
            fakeLoadEverything(tc.fakeLoadEverything);
            if (tc.commitInfoStoreData !== undefined) updateCommitInfo.set(tc.commitInfoStoreData);

            const { container } = render(
                <MemoryRouter initialEntries={['/ui/commits/' + tc.commitHash]}>
                    <Routes>
                        <Route
                            path={'/ui/commits/' + (tc.commitHash !== '' ? ':commit' : '')}
                            element={<CommitInfoPage />}
                        />
                    </Routes>
                </MemoryRouter>
            );

            expect(container.getElementsByClassName('spinner')).toHaveLength(tc.expectedSpinnerCount);
            expect(container.getElementsByClassName('main-content')).toHaveLength(tc.expectedMainContentCount);

            for (const expectedString of tc.expectedText) expect(container.textContent).toContain(expectedString);
        });
    });
});
