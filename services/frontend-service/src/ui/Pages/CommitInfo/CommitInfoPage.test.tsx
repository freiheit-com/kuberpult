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

// const mockCommitInfo = jest.fn();
// jest.mock('../../components/CommitInfo/CommitInfo', () => (props) => {
//     mockCommitInfo(props);
//     return <h1>xyzxyz</h1>;
// });

describe('CommitPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <CommitInfoPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    interface dataEnvT {
        name: string;
        loaded: boolean;
        expectedNumMainContent: number;
        expectedNumSpinner: number;
    }
    const sampleEnvData: dataEnvT[] = [
        {
            name: 'renders main',
            loaded: true,
            expectedNumMainContent: 1,
            expectedNumSpinner: 0,
        },
        {
            name: 'renders spinner',
            loaded: false,
            expectedNumMainContent: 0,
            expectedNumSpinner: 1,
        },
    ];
    describe.each(sampleEnvData)(`Renders CommitPage Spinner or Content`, (testcase) => {
        it(testcase.name, () => {
            fakeLoadEverything(testcase.loaded);
            const { container } = getWrapper();
            expect(container.getElementsByClassName('main-content')).toHaveLength(testcase.expectedNumMainContent);
            expect(container.getElementsByClassName('spinner')).toHaveLength(testcase.expectedNumSpinner);
        });
    });
});

test('Commit info page shows an error when the commit ID is not provided in the URL', () => {
    fakeLoadEverything(true);
    const { container } = render(
        <MemoryRouter initialEntries={['/ui/commits/']}>
            <Routes>
                <Route path="/ui/commits/" element={<CommitInfoPage />} />
            </Routes>
        </MemoryRouter>
    );
    expect(container.textContent).toContain('commit ID not provided');
});

test('Commit info page shows a spinner when waiting for the server to respond', () => {
    fakeLoadEverything(true);
    updateCommitInfo.set({ response: undefined, commitInfoReady: CommitInfoState.LOADING });
    const { container } = render(
        <MemoryRouter initialEntries={['/ui/commits/helloooo']}>
            <Routes>
                <Route path="/ui/commits/:commit" element={<CommitInfoPage />} />
            </Routes>
        </MemoryRouter>
    );

    expect(container.getElementsByClassName('spinner')).not.toHaveLength(0);
    expect(container.textContent).toContain('Loading commit info');
});

test('Commit info page shows an error message when the backend returns an error', () => {
    fakeLoadEverything(true);
    updateCommitInfo.set({ response: undefined, commitInfoReady: CommitInfoState.ERROR });

    const { container } = render(
        <MemoryRouter initialEntries={['/ui/commits/helloooo']}>
            <Routes>
                <Route path="/ui/commits/:commit" element={<CommitInfoPage />} />
            </Routes>
        </MemoryRouter>
    );

    expect(container.textContent).toContain('Backend error');
});

test('Commit info page shows displays commit info when everything is okay', () => {
    fakeLoadEverything(true);
    updateCommitInfo.set({
        response: {
            touchedApps: ['google', 'windows'],
            commitMessage: `Add google to windows

Commit message body line 1
Commit message body line 2`,
        },
        commitInfoReady: CommitInfoState.READY,
    });

    const { container } = render(
        <MemoryRouter initialEntries={['/ui/commits/helloooo']}>
            <Routes>
                <Route path="/ui/commits/:commit" element={<CommitInfoPage />} />
            </Routes>
        </MemoryRouter>
    );

    expect(container.textContent).toContain('Commit Add google to windows');
});
