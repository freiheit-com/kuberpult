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
                        eslId: 1,
                        createdAt: new Date('2024-02-09T09:46:00.000Z'),
                        eventType: 'EvtCreateApplicationVersion',
                        json: '{"version": 1, "app": "test-app-name"}',
                    },
                    {
                        eslId: 2,
                        createdAt: new Date('2024-02-10T09:46:00.000Z'),
                        eventType: 'EvtDeployApplication',
                        json: '{"app": "test-app-name", "environment": "dev"}',
                    },
                ],
            },
            expectedEslsTable: {
                head: ['EslId:', 'Date:', 'Event Type:', 'Json:'],
                body: [
                    [
                        '1',
                        '2024-02-09T09:46:00',
                        'EvtCreateApplicationVersion',
                        '{"version": 1, "app": "test-app-name"}',
                    ],
                    [
                        '2',
                        '2024-02-10T09:46:00',
                        'EvtDeployApplication',
                        '{"app": "test-app-name", "environment": "dev"}',
                    ],
                ],
            },
        },
        {
            selectedTimezone: 'local',
            failedEslsResponse: {
                failedEsls: [
                    {
                        eslId: 1,
                        createdAt: new Date('2024-02-09T11:20:00Z'),
                        eventType: 'EvtCreateApplicationVersion',
                        json: '{"version": 1, "app": "test-app-name"}',
                    },
                ],
            },
            expectedEslsTable: {
                head: ['EslId:', 'Date:', 'Event Type:', 'Json:'],
                body: [
                    [
                        '1',
                        '2024-02-09T12:20:00',
                        'EvtCreateApplicationVersion',
                        '{"version": 1, "app": "test-app-name"}',
                    ],
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
        expect(container.getElementsByTagName('h1')[0]).toHaveTextContent('Failed Esls List:');
        const selectTimezoneElement = container.getElementsByClassName('select-timezone')[0];
        fireEvent.change(selectTimezoneElement, { target: { value: testCase.selectedTimezone } });

        const tables = container.getElementsByTagName('table');

        expect(tables.length).toEqual(1);
        const actualEslsTable = tables[0];

        verifyTable(actualEslsTable, testCase.expectedEslsTable);
    }
});
