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
import { act, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Environment, EnvironmentGroup, Priority, ProductSummary, TagData } from '../../../api/api';
import { ProductVersion, TableFiltered } from './ProductVersion';
import { Spy } from 'spy4js';

const mock_UseEnvGroups = Spy('envGroup');
const mock_UseTags = Spy('Overview');

jest.mock('../../utils/store', () => ({
    refreshTags() {
        return {};
    },
    useEnvironmentGroups() {
        return mock_UseEnvGroups();
    },
    useTags() {
        return mock_UseTags();
    },
    useEnvironments() {
        return [];
    },
    showSnackbarError() {},
}));

jest.mock('../../utils/Links', () => ({
    DisplayManifestLink() {
        return <></>;
    },
    DisplaySourceLink() {
        return <></>;
    },
}));

const mockGetProductSummary = jest.fn();

jest.mock('../../utils/GrpcApi', () => ({
    get useApi() {
        return {
            gitService: () => ({
                GetProductSummary: () => mockGetProductSummary(),
            }),
        };
    },
}));
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
                    priority: Priority.UNRECOGNIZED,
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
                    priority: Priority.UNRECOGNIZED,
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
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        },
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        // given
        it(testCase.name, async () => {
            // replicate api calls
            mock_UseEnvGroups.returns(testCase.environmentGroups);
            mock_UseTags.returns({ response: { tagData: testCase.tags }, tagsReady: true });
            mockGetProductSummary.mockResolvedValue({ productSummary: testCase.productSummary });
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);
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
            const releaseTrainButton = screen.queryByText('Run Release Train');
            if (testCase.tags.length > 0) {
                expect(releaseTrainButton).toBeInTheDocument();
            }
        });
    });
});

describe('Test table filtering', () => {
    type TestData = {
        name: string;
        productSummary: ProductSummary[];
        teams: string[];
        expectedApps: string[];
        filteredApps: string[];
    };
    const data: TestData[] = [
        {
            name: 'no rows to display',
            productSummary: [],
            teams: [],
            expectedApps: [],
            filteredApps: [],
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
            expectedApps: ['testing-app'],
            filteredApps: [],
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
                    app: 'testing-app2',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
            ],
            teams: ['sre-team'],
            expectedApps: ['testing-app'],
            filteredApps: ['testing-app2'],
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
            expectedApps: ['testing-app', 'testing-app 2', 'testing-app 3'],
            filteredApps: [],
        },
        {
            name: 'bigger example',
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
                {
                    app: 'testing-app 4',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'another',
                },
                {
                    app: 'testing-app 5',
                    version: '4',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'last team',
                },
            ],
            teams: ['sre-team', 'others'],
            expectedApps: ['testing-app', 'testing-app 2', 'testing-app 3'],
            filteredApps: ['testing-app 4', 'testing-app 5'],
        },
    ];
    describe.each(data)(`Displays Product Version Table`, (testCase) => {
        it(testCase.name, () => {
            render(<TableFiltered productSummary={testCase.productSummary} teams={testCase.teams} />);
            expect(document.querySelector('.table')?.textContent).toContain('App Name');
            for (let i = 0; i < testCase.expectedApps.length; i++) {
                expect(document.querySelector('.table')?.textContent).toContain(testCase.expectedApps[i]);
            }
            for (let i = 0; i < testCase.filteredApps.length; i++) {
                expect(screen.queryByText(testCase.filteredApps[i])).not.toBeInTheDocument();
            }
        });
    });
});
