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

import { MemoryRouter } from 'react-router-dom';
import { ReleaseTrainPrognosis } from '../../components/ReleaseTrainPrognosis/ReleaseTrainPrognosis';
import { render } from '@testing-library/react';
import {
    Application,
    GetReleaseTrainPrognosisResponse,
    ReleaseTrainAppSkipCause,
    ReleaseTrainEnvSkipCause,
    UndeploySummary,
} from '../../../api/api';
import { UpdateOverview } from '../../utils/store';

test('ReleaseTrain component does not render anything if the response is undefined', () => {
    const { container } = render(
        <MemoryRouter>
            <ReleaseTrainPrognosis releaseTrainPrognosis={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
});

describe('ReleaseTrain component renders release train prognosis when the response is valid', () => {
    type Table = {
        head: string[];
        // NOTE: newlines, if there are any, will effectively be removed, since they will be checked using .toHaveTextContent
        body: string[][];
    };

    type EnvReleaseTrainPrognosisModel = {
        headerText: string;
        body: { type: 'text'; content: string } | { type: 'table'; content: Table };
    };

    type TestCase = {
        name: string;
        releaseTrainPrognosis: GetReleaseTrainPrognosisResponse;
        expectedPageContent: EnvReleaseTrainPrognosisModel[];
        applicationsOverview: {
            [key: string]: Application;
        };
    };

    const testCases: TestCase[] = [
        {
            name: 'prognosis with skipped environments and skipped apps',
            releaseTrainPrognosis: {
                envsPrognoses: {
                    'env-1': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
                        },
                    },
                    'env-2': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_NO_UPSTREAM,
                        },
                    },
                    'env-3': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV,
                        },
                    },
                    'env-4': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_IS_LOCKED,
                        },
                    },
                    'env-5': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.UNRECOGNIZED,
                        },
                    },
                    'env-6': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.UPSTREAM_ENV_CONFIG_NOT_FOUND,
                        },
                    },
                    'env-7': {
                        outcome: {
                            $case: 'appsPrognoses',
                            appsPrognoses: {
                                prognoses: {
                                    'app-1': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.APP_ALREADY_IN_UPSTREAM_VERSION,
                                        },
                                    },
                                    'app-2': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.APP_DOES_NOT_EXIST_IN_ENV,
                                        },
                                    },
                                    'app-3': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.APP_HAS_NO_VERSION_IN_UPSTREAM_ENV,
                                        },
                                    },
                                    'app-4': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.APP_IS_LOCKED,
                                        },
                                    },
                                    'app-5': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.APP_IS_LOCKED_BY_ENV,
                                        },
                                    },
                                    'app-6': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.TEAM_IS_LOCKED,
                                        },
                                    },
                                    'app-7': {
                                        outcome: {
                                            $case: 'skipCause',
                                            skipCause: ReleaseTrainAppSkipCause.UNRECOGNIZED,
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
            expectedPageContent: [
                {
                    headerText: 'Prognosis for release train on environment env-1',
                    body: {
                        type: 'text',
                        content:
                            'Release train on this environment is skipped because it both has an upstream environment and is set as latest.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-2',
                    body: {
                        type: 'text',
                        content: 'Release train on this environment is skipped because it has no upstream configured.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-3',
                    body: {
                        type: 'text',
                        content:
                            'Release train on this environment is skipped because it neither has an upstream environment configured nor is marked as latest.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-4',
                    body: {
                        type: 'text',
                        content: 'Release train on this environment is skipped because it is locked.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-5',
                    body: {
                        type: 'text',
                        content: 'Release train on this environment is skipped due to an unknown reason.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-6',
                    body: {
                        type: 'text',
                        content:
                            'Release train on this environment is skipped because no configuration was found for it in the manifest repository.',
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-7',
                    body: {
                        type: 'table',
                        content: {
                            head: ['Application', 'Outcome'],
                            body: [
                                [
                                    'app-1',
                                    'Application release is skipped because it is already in the upstream version.',
                                ],
                                [
                                    'app-2',
                                    'Application release is skipped because it does not exist in the environment.',
                                ],
                                [
                                    'app-3',
                                    'Application release is skipped because it does not have a version in the upstream environment.',
                                ],
                                ['app-4', 'Application release is skipped because it is locked.'],
                                [
                                    'app-5',
                                    "Application release is skipped because there's an environment lock where this application is getting deployed.",
                                ],
                                ['app-6', 'Application release is skipped due to a team lock'],
                                ['app-7', 'Application release it skipped due to an unrecognized reason'],
                            ],
                        },
                    },
                },
            ],
            applicationsOverview: {},
        },
        {
            name: 'prognosis with some deployed apps',
            applicationsOverview: {
                'app-1': {
                    name: 'app-1',
                    sourceRepoUrl: 'some url',
                    team: 'some team',
                    undeploySummary: UndeploySummary.UNRECOGNIZED,
                    warnings: [],
                    releases: [
                        {
                            version: 1,
                            displayVersion: 'some display version',
                            prNumber: 'some pr number',
                            sourceAuthor: 'some source author',
                            sourceCommitId: 'aaaaaaaaaabbbbbbbbbbccccccccccdddddddddd',
                            sourceMessage: 'some source message',
                            undeployVersion: false,
                            isMinor: false,
                            isPrepublish: false,
                        },
                    ],
                },
                'app-3': {
                    name: 'app-3',
                    sourceRepoUrl: 'some url',
                    team: 'some team',
                    undeploySummary: UndeploySummary.UNRECOGNIZED,
                    warnings: [],
                    releases: [
                        {
                            version: 1,
                            displayVersion: 'some display version',
                            prNumber: 'some pr number',
                            sourceAuthor: 'some source author',
                            sourceCommitId: '0000000000111111111122222222223333333333',
                            sourceMessage: 'some source message',
                            undeployVersion: false,
                            isMinor: false,
                            isPrepublish: false,
                        },
                    ],
                },
            },
            releaseTrainPrognosis: {
                envsPrognoses: {
                    'env-1': {
                        outcome: {
                            $case: 'appsPrognoses',
                            appsPrognoses: {
                                prognoses: {
                                    'app-1': {
                                        outcome: {
                                            $case: 'deployedVersion',
                                            deployedVersion: 1,
                                        },
                                    },
                                    'app-2': {
                                        outcome: {
                                            $case: 'deployedVersion',
                                            deployedVersion: 2,
                                        },
                                    },
                                },
                            },
                        },
                    },
                    'env-2': {
                        outcome: {
                            $case: 'appsPrognoses',
                            appsPrognoses: {
                                prognoses: {
                                    'app-3': {
                                        outcome: {
                                            $case: 'deployedVersion',
                                            deployedVersion: 1,
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
            expectedPageContent: [
                {
                    headerText: 'Prognosis for release train on environment env-1',
                    body: {
                        type: 'table',
                        content: {
                            head: ['Application', 'Outcome'],
                            body: [
                                ['app-1', 'Commit aaaaaaaaaabbbbbbbbbbccccccccccdddddddddd will be released'],
                                ['app-2', 'Commit loading will be released.'], // application version is not in the overview store, so "loading" will be displayed
                            ],
                        },
                    },
                },
                {
                    headerText: 'Prognosis for release train on environment env-2',
                    body: {
                        type: 'table',
                        content: {
                            head: ['Application', 'Outcome'],
                            body: [['app-3', 'Commit 0000000000111111111122222222223333333333 will be released.']],
                        },
                    },
                },
            ],
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
        test(testCase.name, () => {
            UpdateOverview.set({
                applications: testCase.applicationsOverview,
            });
            const { container } = render(
                <MemoryRouter>
                    <ReleaseTrainPrognosis releaseTrainPrognosis={testCase.releaseTrainPrognosis} />
                </MemoryRouter>
            );

            const mainContents = container.getElementsByClassName('main-content');
            expect(mainContents.length).toEqual(1); // there should be one main-content

            const mainContent = mainContents.item(0);
            if (mainContent === null) {
                throw new Error('main content should not be null');
            }

            // there should be one div for ever environment
            expect(mainContent.children.length).toEqual(testCase.expectedPageContent.length);

            const N = testCase.expectedPageContent.length;
            for (let i = 0; i < N; i++) {
                const expectedSection = testCase.expectedPageContent[i];
                const envSection = mainContent.children.item(i);

                if (envSection === null) {
                    throw new Error('environment section should not be null');
                }

                // there should be 2 children, one for the header, one for the content
                expect(envSection.children.length).toEqual(2);

                // check header
                const header = envSection.children.item(0);
                if (header === null) throw new Error('header should not be null');
                expect(header.tagName).toEqual('H1');
                expect(header.textContent).toEqual(expectedSection.headerText);

                // check body
                const body = envSection.children.item(1);
                if (body === null) throw new Error('body should not be null');
                if (expectedSection.body.type === 'text') {
                    expect(body.tagName).toEqual('P');
                    expect(body.textContent).toEqual(expectedSection.body.content);
                } else {
                    expect(body.tagName).toEqual('TABLE');

                    // eslint-disable-next-line no-type-assertion/no-type-assertion
                    const table = body as HTMLTableElement;
                    verifyTable(table, expectedSection.body.content);
                }
            }
        });
    }
});
