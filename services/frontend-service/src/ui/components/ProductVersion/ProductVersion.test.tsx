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
import { act, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Environment, EnvironmentGroup, Priority, ProductSummary, TagData } from '../../../api/api';
import { ProductVersion, TableFiltered } from './ProductVersion';
import { Spy } from 'spy4js';
import type { TagsWithFilter } from '../../utils/store';
import { TagResponse } from '../../utils/store';

const mock_UseEnvGroups = Spy('envGroup');
const mock_UseTags = Spy('Overview');
const mock_FrontendConfig = Spy('FrontendConfig');

jest.mock('../../utils/store', () => ({
    refreshTags() {
        return {};
    },
    useFrontendConfig() {
        return mock_FrontendConfig();
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

    TagResponse: {
        LOADING: 0,
        READY: 1,
        ERROR: 2,
    },
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
            productSummaryService: () => ({
                GetProductSummary: (req: unknown) => mockGetProductSummary(req),
            }),
        };
    },
}));
const sampleEnvsA: Environment[] = [
    {
        name: 'tester',
        distanceToUpstream: 0,
        priority: Priority.UPSTREAM,
    },
];

// Shared mock inputs so each test only has to declare what actually varies (the tags).
const standardEnvGroups: EnvironmentGroup[] = [
    {
        environments: [sampleEnvsA[0]],
        distanceToUpstream: 1,
        environmentGroupName: 'g1',
        priority: Priority.UNRECOGNIZED,
    },
];
const standardFrontendConfig = {
    configsReady: true,
    configs: {
        sourceRepoUrl: '',
        manifestRepoUrl: '',
        branch: '',
        kuberpultVersion: '0',
        revisionsEnabled: false,
    },
};

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
                    revision: '0',
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
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'tester',
                    version: '10',
                    revision: '0',
                    commitId: '4565',
                    displayVersion: '',
                    environment: 'dev',
                    team: '',
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
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        // given
        it(testCase.name, async () => {
            // replicate api calls
            mock_UseEnvGroups.returns(testCase.environmentGroups);
            const useTagsResponse: TagsWithFilter = {
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: [{ tag: 'test-tag-1', commitId: 'sha-123' }],
            };
            mock_UseTags.returns(useTagsResponse);
            mockGetProductSummary.mockResolvedValue({ productSummary: testCase.productSummary });
            mock_FrontendConfig.returns({
                configsReady: true,
                configs: {
                    sourceRepoUrl: '',
                    manifestRepoUrl: '',
                    branch: '',
                    kuberpultVersion: '0',
                    revisionsEnabled: false,
                },
            });
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
                expect(document.querySelector('.warning-message')?.textContent).toContain('There are no git tags ');
            }
            const releaseTrainButton = screen.queryByText('Run Release Train');
            if (testCase.tags.length > 0) {
                expect(releaseTrainButton).toBeInTheDocument();
            }
        });
    });
});

const datedTags: TagData[] = [
    { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
    { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
    { commitId: 'ccc333', tag: 'refs/tags/gamma', commitDate: new Date('2022-03-10T00:00:00Z') },
];

describe('Default tag selection', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        expectedSelectedTag: string;
    };
    const data: TestData[] = [
        {
            name: 'selects the tag with the most recent commit date',
            tags: datedTags,
            expectedSelectedTag: 'beta',
        },
    ];
    describe.each(data)(`Displays the selected tag in the results section`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: testCase.tags,
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            // The latest tag is shown in the results section, outside the dropdown.
            expect(screen.getByTestId('selected_tag').textContent).toContain(testCase.expectedSelectedTag);
            // The release train button lives in the results section.
            expect(screen.getByText('Run Release Train')).toBeInTheDocument();
            // Filters start empty.
            expect(screen.getByTestId('tag_search')).toHaveValue('');
            expect(screen.getByTestId('date_search')).toHaveValue('');
        });
    });
});

describe('Selecting a tag updates the results', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        filterText: string;
        selectCommitId: string;
        initialTableApp: string;
        // Options that must stay selectable after filtering (incl. the still-selected tag).
        requiredOptions: string[];
        expectedTableApp: string;
        absentTableApp: string;
        expectedBanner: string;
    };
    const data: TestData[] = [
        {
            name: 'refetches for a tag picked after filtering it down',
            tags: datedTags,
            filterText: 'alpha',
            selectCommitId: 'aaa111',
            initialTableApp: 'app-for-bbb222',
            requiredOptions: ['bbb222', 'aaa111'],
            expectedTableApp: 'app-for-aaa111',
            absentTableApp: 'app-for-bbb222',
            expectedBanner: 'alpha',
        },
    ];
    describe.each(data)(`Refetches the product summary for the new selection`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: testCase.tags,
            });
            mockGetProductSummary.mockImplementation((req: { manifestRepoCommitHash: string }) =>
                Promise.resolve({
                    productSummary: [
                        {
                            app: 'app-for-' + req.manifestRepoCommitHash,
                            version: '4',
                            revision: '0',
                            commitId: req.manifestRepoCommitHash,
                            displayVersion: 'v1.2.3',
                            environment: 'dev',
                            team: '',
                        },
                    ],
                })
            );
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            // Initial load defaults to the latest tag.
            expect(document.querySelector('.table')?.textContent).toContain(testCase.initialTableApp);

            // Narrow the dropdown via the search filter first, then select the narrowed tag.
            await act(async () => {
                fireEvent.change(screen.getByTestId('tag_search'), { target: { value: testCase.filterText } });
                await global.nextTick();
            });
            // Even when the filter excludes it, the currently selected tag must remain an option so
            // the select keeps displaying the real selection instead of silently jumping to the
            // first filtered option.
            const options = Array.from(screen.getByTestId('drop_down').querySelectorAll('option')).map((o) =>
                o.getAttribute('value')
            );
            for (const required of testCase.requiredOptions) {
                expect(options).toContain(required);
            }

            await act(async () => {
                fireEvent.change(screen.getByTestId('drop_down'), { target: { value: testCase.selectCommitId } });
                await global.nextTick();
            });

            // The results table should now show the newly selected tag's data.
            expect(document.querySelector('.table')?.textContent).toContain(testCase.expectedTableApp);
            expect(document.querySelector('.table')?.textContent).not.toContain(testCase.absentTableApp);
            // ...and the banner should reflect it.
            expect(screen.getByTestId('selected_tag').textContent).toContain(testCase.expectedBanner);
        });
    });
});

describe('Tag selection from the url (page refresh)', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        urlTag: string;
        expectedBanner: string;
    };
    const data: TestData[] = [
        {
            name: 'keeps a non-latest url tag selected instead of defaulting to the latest',
            tags: datedTags,
            urlTag: 'aaa111',
            expectedBanner: 'alpha',
        },
        {
            name: 'keeps the oldest url tag selected instead of the first list element',
            tags: datedTags,
            urlTag: 'ccc333',
            expectedBanner: 'gamma',
        },
    ];
    describe.each(data)(`Restores the selection from the url`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: testCase.tags,
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter initialEntries={['/?tag=' + testCase.urlTag]}>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            expect(screen.getByTestId('selected_tag').textContent).toContain(testCase.expectedBanner);
            expect(screen.getByTestId('drop_down')).toHaveValue(testCase.urlTag);
            expect(mockGetProductSummary).toHaveBeenLastCalledWith(
                expect.objectContaining({ manifestRepoCommitHash: testCase.urlTag })
            );
        });
    });
});

describe('Filters restore from the url on load (survive a refresh)', () => {
    type TestData = {
        name: string;
        urlQuery: string;
        expectedTagFilter: string;
        expectedDateFilter: string;
    };
    const data: TestData[] = [
        {
            name: 'both filters',
            urlQuery: 'tagFilter=alph&dateFilter=2023',
            expectedTagFilter: 'alph',
            expectedDateFilter: '2023',
        },
        {
            name: 'only the tag filter',
            urlQuery: 'tagFilter=beta',
            expectedTagFilter: 'beta',
            expectedDateFilter: '',
        },
        {
            name: 'only the date filter',
            urlQuery: 'dateFilter=2024',
            expectedTagFilter: '',
            expectedDateFilter: '2024',
        },
        {
            name: 'no filters',
            urlQuery: '',
            expectedTagFilter: '',
            expectedDateFilter: '',
        },
    ];
    describe.each(data)(`Populates the filter inputs`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: datedTags }, tagsReady: TagResponse.READY },
                filteredTagData: datedTags,
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            const query = ['tag=aaa111', testCase.urlQuery].filter((part) => part !== '').join('&');
            render(
                <MemoryRouter initialEntries={['/?' + query]}>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            expect(screen.getByTestId('tag_search')).toHaveValue(testCase.expectedTagFilter);
            expect(screen.getByTestId('date_search')).toHaveValue(testCase.expectedDateFilter);
        });
    });
});

describe('Changing a filter does not refetch the product summary', () => {
    type TestData = {
        name: string;
        filterTestId: string;
        newValue: string;
    };
    const data: TestData[] = [
        { name: 'tag filter', filterTestId: 'tag_search', newValue: 'beta' },
        { name: 'date filter', filterTestId: 'date_search', newValue: '2024' },
    ];
    describe.each(data)(`Leaves the results request untouched`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: datedTags }, tagsReady: TagResponse.READY },
                filteredTagData: datedTags,
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter initialEntries={['/?tag=aaa111']}>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);
            // The initial load fetches the product summary exactly once (for the tag in the url).
            expect(mockGetProductSummary).toHaveBeenCalledTimes(1);

            await act(async () => {
                fireEvent.change(screen.getByTestId(testCase.filterTestId), { target: { value: testCase.newValue } });
                await global.nextTick();
            });

            // Filtering only narrows the dropdown; it must not trigger another product summary request.
            expect(mockGetProductSummary).toHaveBeenCalledTimes(1);
        });
    });
});

describe('Tag dropdown ordering', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        expectedOrder: string[];
    };
    const data: TestData[] = [
        {
            name: 'lists tags latest first, oldest last',
            // Intentionally provided out of order.
            tags: [
                { commitId: 'mid', tag: 'refs/tags/mid', commitDate: new Date('2023-06-01T00:00:00Z') },
                { commitId: 'newest', tag: 'refs/tags/newest', commitDate: new Date('2024-12-31T00:00:00Z') },
                { commitId: 'oldest', tag: 'refs/tags/oldest', commitDate: new Date('2022-01-01T00:00:00Z') },
            ],
            expectedOrder: ['newest', 'mid', 'oldest'],
        },
        {
            name: 'sinks tags without a timestamp to the bottom',
            tags: [
                { commitId: 'newest', tag: 'refs/tags/newest', commitDate: new Date('2024-12-31T00:00:00Z') },
                { commitId: 'undated', tag: 'refs/tags/undated' },
                { commitId: 'older', tag: 'refs/tags/older', commitDate: new Date('2022-01-01T00:00:00Z') },
            ],
            expectedOrder: ['newest', 'older', 'undated'],
        },
    ];
    describe.each(data)(`Orders the dropdown options`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: testCase.tags,
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            // The placeholder option is first; the real tags follow newest -> oldest.
            const optionValues = Array.from(screen.getByTestId('drop_down').querySelectorAll('option'))
                .map((o) => o.getAttribute('value'))
                .filter((v) => v !== 'default');
            expect(optionValues).toEqual(testCase.expectedOrder);
        });
    });
});

describe('Tag search filtering', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        search: string;
        expectedTags: string[];
        hiddenTags: string[];
    };
    const data: TestData[] = [
        {
            name: 'empty search shows all tags',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha' },
                { commitId: 'bbb222', tag: 'refs/tags/beta' },
            ],
            search: '',
            expectedTags: ['alpha', 'beta'],
            hiddenTags: [],
        },
        {
            name: 'filters by tag name',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha' },
                { commitId: 'bbb222', tag: 'refs/tags/beta' },
            ],
            search: 'alph',
            expectedTags: ['alpha'],
            hiddenTags: ['beta'],
        },
        {
            name: 'filters by commit id',
            tags: [
                { commitId: 'deadbeef', tag: 'refs/tags/alpha' },
                { commitId: 'cafef00d', tag: 'refs/tags/beta' },
            ],
            search: 'cafe',
            expectedTags: ['beta'],
            hiddenTags: ['alpha'],
        },
        {
            name: 'filtering is case insensitive',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/Alpha' },
                { commitId: 'bbb222', tag: 'refs/tags/beta' },
            ],
            search: 'ALPHA',
            expectedTags: ['Alpha'],
            hiddenTags: ['beta'],
        },
    ];
    describe.each(data)(`Filters the tag dropdown`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: [{ tag: 'test-tag-1', commitId: 'sha-123' }],
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            fireEvent.change(screen.getByTestId('tag_search'), { target: { value: testCase.search } });

            const dropDownText = document.querySelector('.drop_down')?.textContent ?? '';
            for (const expected of testCase.expectedTags) {
                expect(dropDownText).toContain(expected);
            }
            for (const hidden of testCase.hiddenTags) {
                expect(dropDownText).not.toContain(hidden);
            }
        });
    });
});

describe('Tag date filtering', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        dateSearch: string;
        expectedTags: string[];
        hiddenTags: string[];
    };
    const data: TestData[] = [
        {
            name: 'empty date search shows all tags',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
                { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
            ],
            dateSearch: '',
            expectedTags: ['alpha', 'beta'],
            hiddenTags: [],
        },
        {
            name: 'filters by full date',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
                { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
            ],
            dateSearch: '2024-06-15',
            expectedTags: ['beta'],
            hiddenTags: ['alpha'],
        },
        {
            name: 'filters by year only',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
                { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
            ],
            dateSearch: '2023',
            expectedTags: ['alpha'],
            hiddenTags: ['beta'],
        },
        {
            name: 'hides tags with a missing timestamp',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
                { commitId: 'bbb222', tag: 'refs/tags/beta' },
            ],
            dateSearch: '2023',
            expectedTags: ['alpha'],
            hiddenTags: ['beta'],
        },
    ];
    describe.each(data)(`Filters the tag dropdown by date`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns(standardEnvGroups);
            mock_UseTags.returns({
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: [{ tag: 'test-tag-1', commitId: 'sha-123' }],
            });
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
            mock_FrontendConfig.returns(standardFrontendConfig);
            render(
                <MemoryRouter>
                    <ProductVersion />
                </MemoryRouter>
            );
            await act(global.nextTick);

            fireEvent.change(screen.getByTestId('date_search'), { target: { value: testCase.dateSearch } });

            const dropDownText = document.querySelector('.drop_down')?.textContent ?? '';
            for (const expected of testCase.expectedTags) {
                expect(dropDownText).toContain(expected);
            }
            for (const hidden of testCase.hiddenTags) {
                expect(dropDownText).not.toContain(hidden);
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
                    revision: '0',
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
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'testing-app2',
                    version: '4',
                    revision: '0',
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
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'testing-app 2',
                    version: '4',
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
                {
                    app: 'testing-app 3',
                    version: '4',
                    revision: '0',
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
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'sre-team',
                },
                {
                    app: 'testing-app 2',
                    version: '4',
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
                {
                    app: 'testing-app 3',
                    version: '4',
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'others',
                },
                {
                    app: 'testing-app 4',
                    version: '4',
                    revision: '0',
                    commitId: '123',
                    displayVersion: 'v1.2.3',
                    environment: 'dev',
                    team: 'another',
                },
                {
                    app: 'testing-app 5',
                    version: '4',
                    revision: '0',
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
