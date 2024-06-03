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
import { GetReleaseTrainPrognosisResponse, ReleaseTrainAppSkipCause, ReleaseTrainEnvSkipCause } from '../../../api/api';

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
    };

    const testCases: TestCase[] = [
        {
            name: 'single environment skipped for some reason',
            releaseTrainPrognosis: {
                envsPrognoses: {
                    'env-1': {
                        outcome: {
                            $case: 'skipCause',
                            skipCause: ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV,
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
