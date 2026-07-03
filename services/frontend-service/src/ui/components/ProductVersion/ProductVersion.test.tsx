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

describe('Default tag selection', () => {
    type TestData = {
        name: string;
        tags: TagData[];
        expectedSelectedTag: string;
    };
    const data: TestData[] = [
        {
            name: 'selects the tag with the most recent commit date',
            tags: [
                { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
                { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
                { commitId: 'ccc333', tag: 'refs/tags/gamma', commitDate: new Date('2022-03-10T00:00:00Z') },
            ],
            expectedSelectedTag: 'beta',
        },
    ];
    describe.each(data)(`Displays the selected tag in the results section`, (testCase) => {
        it(testCase.name, async () => {
            mock_UseEnvGroups.returns([
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                    priority: Priority.UNRECOGNIZED,
                },
            ]);
            const useTagsResponse: TagsWithFilter = {
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: testCase.tags,
            };
            mock_UseTags.returns(useTagsResponse);
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
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
    it('refetches the product summary for the newly selected tag', async () => {
        const tags: TagData[] = [
            { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
            { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
        ];
        mock_UseEnvGroups.returns([
            {
                environments: [sampleEnvsA[0]],
                distanceToUpstream: 1,
                environmentGroupName: 'g1',
                priority: Priority.UNRECOGNIZED,
            },
        ]);
        const useTagsResponse: TagsWithFilter = {
            tagsResponse: { response: { tagData: tags }, tagsReady: TagResponse.READY },
            filteredTagData: tags,
        };
        mock_UseTags.returns(useTagsResponse);
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

        // Initial load defaults to the latest tag (beta / bbb222).
        expect(document.querySelector('.table')?.textContent).toContain('app-for-bbb222');

        // Narrow the dropdown via the search filter first, then select the narrowed tag.
        await act(async () => {
            fireEvent.change(screen.getByTestId('tag_search'), { target: { value: 'alpha' } });
            await global.nextTick();
        });
        // Even though the filter excludes it, the currently selected tag (beta) must remain an
        // option so the select keeps displaying the real selection instead of silently jumping to
        // the first filtered option.
        const options = Array.from(screen.getByTestId('drop_down').querySelectorAll('option')).map((o) =>
            o.getAttribute('value')
        );
        expect(options).toContain('bbb222');
        expect(options).toContain('aaa111');

        await act(async () => {
            fireEvent.change(screen.getByTestId('drop_down'), { target: { value: 'aaa111' } });
            await global.nextTick();
        });

        // The results table should now show the newly selected tag's data.
        expect(document.querySelector('.table')?.textContent).toContain('app-for-aaa111');
        expect(document.querySelector('.table')?.textContent).not.toContain('app-for-bbb222');
        // ...and the banner should reflect it.
        expect(screen.getByTestId('selected_tag').textContent).toContain('alpha');
    });
});

describe('Tag selection from the url (page refresh)', () => {
    it('keeps the tag from the url selected instead of falling back to the first/latest', async () => {
        const tags: TagData[] = [
            { commitId: 'aaa111', tag: 'refs/tags/alpha', commitDate: new Date('2023-01-01T00:00:00Z') },
            { commitId: 'bbb222', tag: 'refs/tags/beta', commitDate: new Date('2024-06-15T00:00:00Z') },
        ];
        mock_UseEnvGroups.returns([
            {
                environments: [sampleEnvsA[0]],
                distanceToUpstream: 1,
                environmentGroupName: 'g1',
                priority: Priority.UNRECOGNIZED,
            },
        ]);
        const useTagsResponse: TagsWithFilter = {
            tagsResponse: { response: { tagData: tags }, tagsReady: TagResponse.READY },
            filteredTagData: tags,
        };
        mock_UseTags.returns(useTagsResponse);
        mockGetProductSummary.mockResolvedValue({ productSummary: [] });
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
        // Simulate a refresh: the url already carries a tag that is NOT the latest one.
        render(
            <MemoryRouter initialEntries={['/?tag=aaa111']}>
                <ProductVersion />
            </MemoryRouter>
        );
        await act(global.nextTick);

        // The selection must reflect the url tag (alpha), not the latest (beta) or the first option.
        expect(screen.getByTestId('selected_tag').textContent).toContain('alpha');
        expect(screen.getByTestId('drop_down')).toHaveValue('aaa111');
        expect(mockGetProductSummary).toHaveBeenLastCalledWith(
            expect.objectContaining({ manifestRepoCommitHash: 'aaa111' })
        );
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
            mock_UseEnvGroups.returns([
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                    priority: Priority.UNRECOGNIZED,
                },
            ]);
            const useTagsResponse: TagsWithFilter = {
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: [{ tag: 'test-tag-1', commitId: 'sha-123' }],
            };
            mock_UseTags.returns(useTagsResponse);
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
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
            mock_UseEnvGroups.returns([
                {
                    environments: [sampleEnvsA[0]],
                    distanceToUpstream: 1,
                    environmentGroupName: 'g1',
                    priority: Priority.UNRECOGNIZED,
                },
            ]);
            const useTagsResponse: TagsWithFilter = {
                tagsResponse: { response: { tagData: testCase.tags }, tagsReady: TagResponse.READY },
                filteredTagData: [{ tag: 'test-tag-1', commitId: 'sha-123' }],
            };
            mock_UseTags.returns(useTagsResponse);
            mockGetProductSummary.mockResolvedValue({ productSummary: [] });
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
