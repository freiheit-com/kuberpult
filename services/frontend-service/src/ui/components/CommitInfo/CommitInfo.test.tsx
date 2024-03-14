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
import { CommitInfo } from './CommitInfo';
import { MemoryRouter } from 'react-router-dom';
import { GetCommitInfoResponse, LockPreventedDeploymentEvent_LockType } from '../../../api/api';

test('CommitInfo component does not render commit info when the response is undefined', () => {
    const { container } = render(
        <MemoryRouter>
            <CommitInfo commitInfo={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
});

test('CommitInfo component renders commit info when the response is valid', () => {
    type Table = {
        head: string[];
        // NOTE: newlines, if there are any, will effectively be removed, since they will be checked using .toHaveTextContent
        body: string[][];
    };

    type TestCase = {
        commitInfo: GetCommitInfoResponse;
        expectedTitle: string;
        expectedCommitDescriptionTable: Table;
        expectedEventsTable: Table;
    };

    const testCases: TestCase[] = [
        {
            commitInfo: {
                commitHash: 'potato',
                commitMessage: `tomato
                
        Commit message body line 1
        Commit message body line 2`,
                touchedApps: ['google', 'windows'],
                nextCommitHash: '',
                previousCommitHash: '',
                events: [
                    {
                        uuid: '00000000-0000-0000-0000-000000000000',
                        createdAt: new Date('2024-02-09T09:46:00Z'),
                        eventType: {
                            $case: 'createReleaseEvent',
                            createReleaseEvent: {
                                environmentNames: ['dev', 'staging'],
                            },
                        },
                    },
                    {
                        uuid: '00000000-0000-0000-0000-000000000001',
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
                        uuid: '00000000-0000-0000-0000-000000000002',
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
                        uuid: '00000000-0000-0000-0000-000000000003',
                        createdAt: new Date('2024-02-12T09:46:00Z'),
                        eventType: {
                            $case: 'deploymentEvent',
                            deploymentEvent: {
                                application: 'app',
                                targetEnvironment: 'staging',
                                releaseTrainSource: {
                                    upstreamEnvironment: 'dev',
                                    targetEnvironmentGroup: 'staging-group',
                                },
                            },
                        },
                    },
                    {
                        uuid: '00000000-0000-0000-0000-000000000004',
                        createdAt: new Date('2024-02-13T09:46:00Z'),
                        eventType: {
                            $case: 'lockPreventedDeploymentEvent',
                            lockPreventedDeploymentEvent: {
                                application: 'app',
                                environment: 'dev',
                                lockMessage: 'locked',
                                lockType: LockPreventedDeploymentEvent_LockType.LOCK_TYPE_ENV,
                            },
                        },
                    },
                    {
                        uuid: '00000000-0000-0000-0000-000000000005',
                        createdAt: new Date('2024-02-13T09:46:00Z'),
                        eventType: {
                            $case: 'replacedByEvent',
                            replacedByEvent: {
                                application: 'app',
                                environment: 'dev',
                                replacedByCommitId: '1234567891011121314ABCD',
                            },
                        },
                    },
                ],
            },
            expectedTitle: 'Commit tomato',
            expectedCommitDescriptionTable: {
                head: ['Commit Hash:', 'Commit Message:', 'Touched apps:'],
                body: [['potato', `tomato Commit message body line 1 Commit message body line 2`, 'google, windows']],
            },
            expectedEventsTable: {
                head: ['Date:', 'Event Description:', 'Environments:'],
                body: [
                    ['2024-02-09T09:46:00', 'received data about this commit for the first time', 'dev, staging'],
                    ['2024-02-10T09:46:00', 'Manual deployment of application app to environment dev', 'dev'],
                    [
                        '2024-02-11T09:46:00',
                        'Release train deployment of application app from environment dev to environment staging',
                        'staging',
                    ],
                    [
                        '2024-02-12T09:46:00',
                        'Release train deployment of application app on environment group staging-group from environment dev to environment staging',
                        'staging',
                    ],
                    [
                        '2024-02-13T09:46:00',
                        'Application app was blocked from deploying due to an environment lock with message "locked"',
                        'dev',
                    ],
                    ['2024-02-13T09:46:00', 'This commit was replaced by 12345678 on dev.', 'dev'],
                ],
            },
        },
    ];

    const verifyTable = (actualTable: HTMLTableElement, expectedTable: Table) => {
        // header verification
        const actualHeaders = actualTable.getElementsByTagName('thead');
        expect(actualHeaders).toHaveLength(1); // there should be 1 header line

        const actualHeadersRows = actualHeaders[0].getElementsByTagName('tr');
        expect(actualHeadersRows).toHaveLength(1); // there should be 1 row in the header line

        const actualHeaderFields = actualHeadersRows[0].getElementsByTagName('th');
        expect(actualHeaderFields).toHaveLength(expectedTable.head.length);

        for (let i = 0; i < actualHeaderFields.length; i++) {
            expect(actualHeaderFields[i].innerHTML).toEqual(expectedTable.head[i]);
        }

        // rows verification
        const actualBody = actualTable.getElementsByTagName('tbody');
        expect(actualBody).toHaveLength(1);

        const actualRows = actualBody[0].getElementsByTagName('tr');
        expect(actualRows).toHaveLength(expectedTable.body.length);

        for (let i = 0; i < actualRows.length; i++) {
            const actualRowFields = actualRows[i].getElementsByTagName('td');
            expect(actualRowFields).toHaveLength(expectedTable.body[i].length);

            for (let j = 0; j < actualHeaderFields.length; j++) {
                expect(actualRowFields[j]).toHaveTextContent(expectedTable.body[i][j]);
            }
        }
    };

    for (const testCase of testCases) {
        const { container } = render(
            <MemoryRouter>
                <CommitInfo commitInfo={testCase.commitInfo} />
            </MemoryRouter>
        );

        // first h1 is "Planned Actions", second h1 is "This page is still in beta", and we need the 3rd
        expect(container.getElementsByTagName('h1').length).toBeGreaterThan(2);
        expect(container.getElementsByTagName('h1')[2]).toHaveTextContent(testCase.expectedTitle);

        const tables = container.getElementsByTagName('table');

        expect(tables.length).toEqual(2); // one table for commit description and one table for events

        const actualCommitDescriptionTable = tables[0];
        const actualEventsTable = tables[1];

        verifyTable(actualCommitDescriptionTable, testCase.expectedCommitDescriptionTable);
        verifyTable(actualEventsTable, testCase.expectedEventsTable);
    }
});
