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
import { render, fireEvent } from '@testing-library/react';
import { EslWarnings } from './EslWarnings';
import { MemoryRouter } from 'react-router-dom';
import { GetFailedEslsResponse } from '../../../api/api';

test('EslWarnings component does not render EslWarnings when the response is undefined', () => {
    const { container } = render(
        <MemoryRouter>
            <EslWarnings failedEsls={undefined} />
        </MemoryRouter>
    );
    expect(container.textContent).toContain('Backend returned empty response');
});

test('EslWarnings component renders Esl Warnings when the response is valid', () => {
    type Table = {
        head: string[];
        body: string[][];
    };

    type TestCase = {
        selectedTimezone: 'local' | 'UTC';
        failedEslsResponse: GetFailedEslsResponse;
        expectedEslsTable: Table;
    };

    const testCases: TestCase[] = [
        {
            selectedTimezone: 'UTC',
            failedEslsResponse: {
                failedEsls: [
                    {
                        eslVersion: 1,
                        createdAt: new Date('2024-02-09T09:46:00.000Z'),
                        eventType: 'EvtCreateApplicationVersion',
                        json: '{"version": 1, "app": "test-app-name"}',
                        reason: 'unexpected error',
                        transformerEslVersion: 12,
                    },
                    {
                        eslVersion: 2,
                        createdAt: new Date('2024-02-10T09:46:00.000Z'),
                        eventType: 'EvtDeployApplication',
                        json: '{"app": "test-app-name", "environment": "dev"}',
                        reason: 'unexpected error',
                        transformerEslVersion: 17,
                    },
                ],
                loadMore: true,
            },
            expectedEslsTable: {
                head: ['Date', 'ID', 'Type', 'Reason', 'Retry', 'Skip'],
                body: [
                    ['2024-02-09T09:46:00', '12', 'EvtCreateApplicationVersion', 'unexpected error', '', ''],
                    ['2024-02-10T09:46:00', '17', 'EvtDeployApplication', 'unexpected error', '', ''],
                ],
            },
        },
        {
            selectedTimezone: 'local',
            failedEslsResponse: {
                failedEsls: [
                    {
                        eslVersion: 1,
                        createdAt: new Date('2024-02-09T11:20:00Z'),
                        eventType: 'EvtCreateApplicationVersion',
                        json: '{"version": 1, "app": "test-app-name"}',
                        reason: 'unknown error',
                        transformerEslVersion: 9,
                    },
                ],
                loadMore: true,
            },
            expectedEslsTable: {
                head: ['Date', 'ID', 'Type', 'Reason', 'Retry', 'Skip'],
                body: [['2024-02-09T12:20:00', '9', 'EvtCreateApplicationVersion', 'unknown error', '', '']],
            },
        },
    ];

    const verifyTable = (actualTable: HTMLTableElement, expectedTable: Table) => {
        // header verification
        const actualHeaders = actualTable.getElementsByTagName('thead');
        expect(actualHeaders).toHaveLength(1); // there should be 1 header line

        const actualHeadersRows = actualHeaders[0].getElementsByTagName('tr');
        expect(actualHeadersRows).toHaveLength(2); // there should be 2 row in the header line (1 for name of table and another for the column names)

        const actualHeaderFields = actualHeadersRows[1].getElementsByClassName('mdc-data-indicator-field');

        for (let i = 0; i < actualHeaderFields.length; i++) {
            expect(actualHeaderFields[i].textContent).toEqual(expectedTable.head[i]);
        }

        // rows verification
        const actualBody = actualTable.getElementsByTagName('tbody');
        expect(actualBody).toHaveLength(1); // Header row

        const actualRows = actualBody[0].getElementsByClassName('lock-display');
        expect(actualRows).toHaveLength(expectedTable.body.length);

        for (let i = 0; i < actualRows.length; i++) {
            const actualElements = actualRows[i].getElementsByClassName('lock-display-info');
            const reason = actualRows[i].getElementsByClassName('lock-display-info-size-limit');
            for (let j = 0; j < actualElements.length; j++) {
                if (j === 3) {
                    expect(reason[0].textContent).toEqual(expectedTable.body[i][j]);
                } else {
                    expect(actualElements[j].textContent).toEqual(expectedTable.body[i][j]);
                }
            }
        }
    };

    for (const testCase of testCases) {
        jest.spyOn(Intl, 'DateTimeFormat').mockImplementation(
            () =>
                // eslint-disable-next-line no-type-assertion/no-type-assertion
                ({
                    resolvedOptions: () => ({ timeZone: 'Europe/Berlin' }),
                }) as Intl.DateTimeFormat
        );
        const { container } = render(
            <MemoryRouter>
                <EslWarnings failedEsls={testCase.failedEslsResponse} />
            </MemoryRouter>
        );

        expect(container.getElementsByTagName('h1').length).toEqual(1);
        expect(container.getElementsByTagName('h1')[0]).toHaveTextContent('Failed ESL Event List:');
        const selectTimezoneElement = container.getElementsByClassName('select-timezone')[0];
        fireEvent.change(selectTimezoneElement, { target: { value: testCase.selectedTimezone } });

        const tables = container.getElementsByTagName('table');

        expect(tables.length).toEqual(1);
        const actualEslsTable = tables[0];

        verifyTable(actualEslsTable, testCase.expectedEslsTable);
    }
});
