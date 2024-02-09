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
import { render, screen } from '@testing-library/react';
import { CommitInfo } from './CommitInfo';
import { MemoryRouter } from 'react-router-dom';
import { GetCommitInfoResponse } from '../../../api/api';

test('CommitInfo component does not render commit info when the response is undefined', () => {
    const { container } = render(
        <MemoryRouter>
            <CommitInfo commitHash={'potato'} commitInfo={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
    expect(container.textContent).not.toContain('potato');
});

test('CommitInfo component renders commit info when the response is valid', () => {
    const commitHash: string = 'potato';
    const commitInfo: GetCommitInfoResponse = {
        commitMessage: `tomato
        
Commit message body line 1
Commit message body line 2`,
        touchedApps: ['google', 'windows'],
        events: [],
    };
    render(
        <MemoryRouter>
            <CommitInfo commitHash={commitHash} commitInfo={commitInfo} />
        </MemoryRouter>
    );

    expect(screen.getAllByRole('heading', { name: 'Commit tomato' })).toHaveLength(1);

    expect(screen.getAllByRole('row', { name: /potato/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /tomato/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /Commit message body line 1/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /Commit message body line 2/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /google, windows/ })).toHaveLength(1);
});
