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
import { GetGitTagsResponse, GetProductSummaryResponse, ProductSummary, TagData } from '../../../api/api';
import { updateSummary, updateTag } from '../../utils/store';
import { ProductVersion } from './ProductVersion';

describe('Product Version Data', () => {
    type TestData = {
        name: string;
        environmentName: string;
        expectedDropDown: string;
        tags: TagData[];
        productSummary: ProductSummary[];
    };
    const data: TestData[] = [
        {
            name: 'No tags to Display',
            environmentName: 'tester',
            tags: [],
            expectedDropDown: 'Select a Tag',
            productSummary: [],
        },
        {
            name: 'tags to Display with summary',
            environmentName: 'tester2',
            tags: [{ commitId: '123', tag: 'refs/tags/dummyTag' }],
            expectedDropDown: 'dummyTag',
            productSummary: [{ app: 'testing-app', version: '4', commitId: '123', displayVersion: 'v1.2.3' }],
        },
        {
            name: 'table to be displayed with multiple rows of data',
            environmentName: 'tester2',
            tags: [
                { commitId: '123', tag: 'refs/tags/dummyTag' },
                { commitId: '859', tag: 'refs/tags/dummyTag2' },
            ],
            expectedDropDown: 'dummyTag',
            productSummary: [
                { app: 'testing-app', version: '4', commitId: '123', displayVersion: 'v1.2.3' },
                { app: 'tester', version: '10', commitId: '4565', displayVersion: '' },
            ],
        },
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        // given
        it(testCase.name, () => {
            // replicate api calls
            const tagsResponse: GetGitTagsResponse = { tagData: testCase.tags };
            updateTag.set(tagsResponse);
            const summaryResponse: GetProductSummaryResponse = { productSummary: testCase.productSummary };
            updateSummary.set(summaryResponse);

            render(
                <MemoryRouter>
                    <ProductVersion environment={testCase.environmentName} />
                </MemoryRouter>
            );
            expect(document.body).toMatchSnapshot();
            expect(document.querySelector('.environment_name')?.textContent).toContain(testCase.environmentName);
            expect(document.querySelector('.drop_down')?.textContent).toContain(testCase.expectedDropDown);

            if (testCase.productSummary.length > 0) {
                expect(document.querySelector('.table')?.textContent).toContain('App/Service Name');
            } else {
                expect(document.querySelector('.page_description')?.textContent).toContain(
                    'This page shows the version'
                );
            }
        });
    });
});
