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
import { MemoryRouter } from 'react-router-dom';
import {
    Environment,
    EnvironmentGroup,
    GetGitTagsResponse,
    GetProductSummaryResponse,
    Priority,
    ProductSummary,
    TagData,
} from '../../../api/api';
import { UpdateOverview, updateSummary, updateTag } from '../../utils/store';
import { ProductVersion, TableFiltered } from './ProductVersion';

const sampleEnvsA: Environment[] = [
    {
        name: 'tester',
        locks: {},
        applications: {},
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
];

describe('Product Version Data', () => {
    type TestData = {
        name: string;
        environmentName: string;
        environmentGroups: EnvironmentGroup[];
        expectedDropDown: string;
        tags: TagData[];
        productSummary: ProductSummary[];
    };
    const data: TestData[] = [
        {
            name: 'No tags to Display',
            environmentName: 'tester',
            tags: [],
            expectedDropDown: '',
            productSummary: [],
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
            ],
        },
        {
            name: 'tags to Display with summary',
            environmentName: 'tester',
            tags: [{ commitId: '123', tag: 'refs/tags/dummyTag' }],
            expectedDropDown: 'dummyTag',
            productSummary: [
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
            ],
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
            ],
        },
        {
            name: 'table to be displayed with multiple rows of data',
            environmentName: 'tester',
            tags: [
                { commitId: '123', tag: 'refs/tags/dummyTag' },
                { commitId: '859', tag: 'refs/tags/dummyTag2' },
            ],
            expectedDropDown: 'dummyTag',
            productSummary: [
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                { app: 'tester', version: '10', commitId: '4565', displayVersion: '', environment: 'dev', team: '' },
            ],
            environmentGroups: [
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                },
            ],
        },
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        // given
        it(testCase.name, () => {
            // replicate api calls
            UpdateOverview.set({
                environmentGroups: testCase.environmentGroups,
            });
            const tagsResponse: GetGitTagsResponse = { tagData: testCase.tags };
            updateTag.set({ response: tagsResponse, tagsReady: true });
            const summaryResponse: GetProductSummaryResponse = { productSummary: testCase.productSummary };
            updateSummary.set({ response: summaryResponse, summaryReady: true });

            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            expect(document.body).toMatchSnapshot();
            if (testCase.expectedDropDown !== '') {
                expect(document.querySelector('.drop_down')?.textContent).toContain(testCase.expectedDropDown);
                expect(document.querySelector('.env_drop_down')?.textContent).toContain(testCase.environmentName);
            }

            if (testCase.productSummary.length > 0) {
                expect(document.querySelector('.table')?.textContent).toContain('App Name');
            } else {
                expect(document.querySelector('.page_description')?.textContent).toContain(
                    'This page shows the version'
                );
            }
        });
    });
});

describe('Test table filtering', () => {
    type TestData = {
        name: string;
        productSummary: ProductSummary[];
        teams: string[];
    };
    const data: TestData[] = [
        {
            name: 'no rows to display',
            productSummary: [],
            teams: [],
        },
        {
            name: 'no teams to filter out',
            productSummary: [
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
            ],
            teams: [],
        },
        {
            name: 'a team to filter out',
            productSummary: [
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
            ],
            teams: ['sre-team'],
        },
        {
            name: 'a team to filter out',
            productSummary: [
                {
                    app: 'testing-app',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'testing-app 2',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
                {
                    app: 'testing-app 3',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
            ],
            teams: ['sre-team', 'others'],
        },
    ];
    describe.each(data)(`Displays Product Version Table`, (testCase) => {
        render(<TableFiltered productSummary={testCase.productSummary} teams={testCase.teams} />);
        expect(document.querySelector('.table')?.textContent).toContain('App Name');
        expect(document.body).toMatchSnapshot();
    });
});
