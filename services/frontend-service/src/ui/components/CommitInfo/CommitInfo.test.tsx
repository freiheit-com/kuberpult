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
            <CommitInfo commitInfo={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
});

test('CommitInfo component renders commit info when the response is valid', () => {
    const commitInfo: GetCommitInfoResponse = {
        commitHash: 'potato',
        commitMessage: `tomato
        
Commit message body line 1
Commit message body line 2`,
        touchedApps: ['google', 'windows'],
        events: [
            {
                createdAt: new Date('2024-02-09T09:46:00Z'),
                eventType: {
                    $case: 'createReleaseEvent',
                    createReleaseEvent: {
                        environmentNames: ['dev', 'staging'],
                    },
                },
            },
            {
                createdAt: new Date('2024-02-10T09:46:00Z'),
                eventType: {
                    $case: 'deploymentEvent',
                    deploymentEvent: {
                        application: 'app',
                        targetEnvironment: 'dev',
                    },
                },
            },
            {
                createdAt: new Date('2024-02-11T09:46:00Z'),
                eventType: {
                    $case: 'deploymentEvent',
                    deploymentEvent: {
                        application: 'app',
                        targetEnvironment: 'staging',
                        releaseTrainSource: {
                            upstreamEnvironment: 'dev',
                        },
                    },
                },
            },
            {
                createdAt: new Date('2024-02-12T09:46:00Z'),
                eventType: {
                    $case: 'deploymentEvent',
                    deploymentEvent: {
                        application: 'app',
                        targetEnvironment: 'staging',
                        releaseTrainSource: {
                            upstreamEnvironment: 'dev',
                            targetGroup: 'staging-group',
                        },
                    },
                },
            },
        ],
    };
    render(
        <MemoryRouter>
            <CommitInfo commitInfo={commitInfo} />
        </MemoryRouter>
    );

    expect(screen.getAllByRole('heading', { name: 'Commit tomato' })).toHaveLength(1);

    expect(screen.getAllByRole('row', { name: /potato/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /tomato/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /Commit message body line 1/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /Commit message body line 2/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /google, windows/ })).toHaveLength(1);

    // checks for first event
    expect(screen.getAllByRole('row', { name: /2024-02-09T09:46:00/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /received data about this commit for the first time/ })).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /dev, staging/ })).toHaveLength(1);

    // checks for second event
    expect(screen.getAllByRole('row', { name: /2024-02-10T09:46:00/ })).toHaveLength(1);
    expect(
        screen.getAllByRole('row', { name: /Manual deployment of application app to environment dev/ })
    ).toHaveLength(1);
    expect(screen.getAllByRole('row', { name: /dev/ })).toHaveLength(4); // there are 3 others

    // checks for third event
    expect(screen.getAllByRole('row', { name: /2024-02-11T09:46:00/ })).toHaveLength(1);
    expect(
        screen.getAllByRole('row', {
            name: /Release train deployment of application app from environment dev to environment staging/,
        })
    ).toHaveLength(1);

    // checks for 4th event
    expect(screen.getAllByRole('row', { name: /2024-02-12T09:46:00/ })).toHaveLength(1);
    expect(
        screen.getAllByRole('row', {
            name: /Release train deployment of application app on environment group staging-group from environment dev to environment staging/,
        })
    ).toHaveLength(1);
});
