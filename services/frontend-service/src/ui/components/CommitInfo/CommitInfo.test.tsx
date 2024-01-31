// import { updateCommitInfo } from "../../utils/store";

import { render, screen } from '@testing-library/react';
import { CommitInfo } from './CommitInfo';
import { MemoryRouter } from 'react-router-dom';
import { GetCommitInfoResponse } from '../../../api/api';

test('CommitInfo component does not render commit info when the response is undefined', () => {
    render(
        <MemoryRouter>
            <CommitInfo commitHash={'potato'} commitInfo={undefined} />
        </MemoryRouter>
    );
    expect(document.body.textContent).toContain('Backend returned empty response');
    expect(document.body.textContent).not.toContain('potato');
});

test('CommitInfo component renders commit info when the response is valid', () => {
    const commitHash: string = 'potato';
    const commitInfo: GetCommitInfoResponse = {
        commitMessage: `tomato
        
Commit message body line 1
Commit message body line 2`,
        touchedApps: ['google', 'windows'],
    };
    render(
        <MemoryRouter>
            <CommitInfo commitHash={commitHash} commitInfo={commitInfo} />
        </MemoryRouter>
    );

    expect(screen.getAllByRole('heading', { name: 'Commit tomato' })).not.toHaveLength(0);

    expect(screen.getAllByRole('row', { name: /potato/ })).not.toHaveLength(0);
    expect(screen.getAllByRole('row', { name: /tomato/ })).not.toHaveLength(0);
    expect(screen.getAllByRole('row', { name: /Commit message body line 1/ })).not.toHaveLength(0);
    expect(screen.getAllByRole('row', { name: /Commit message body line 2/ })).not.toHaveLength(0);
    expect(screen.getAllByRole('row', { name: /google, windows/ })).not.toHaveLength(0);
});
