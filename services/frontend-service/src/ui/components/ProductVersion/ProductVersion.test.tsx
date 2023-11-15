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
import { screen, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { GetGitTagsResponse, TagData } from '../../../api/api';
import { updateTag } from '../../utils/store';
import { ProductVersion } from './ProductVersion';

describe('Product Version Data', () => {
    type TestData = {
        name: string;
        environmentName: string;
        expectedDropDown: string;
        tags: TagData[];
    };
    const data: TestData[] = [
        {
            name: 'No tags to Display',
            environmentName: 'tester',
            tags: [],
            expectedDropDown: 'Select a Tag',
        },
        {
            name: 'tags to Display',
            environmentName: 'tester2',
            tags: [{ commitId: '123', tag: 'refs/tags/dummyTag' }],
            expectedDropDown: 'dummyTag',
        },
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        // given
        it(testCase.name, () => {
            var tagsResponse: GetGitTagsResponse = { tagData: testCase.tags };
            updateTag.set(tagsResponse);
            render(
                <MemoryRouter>
                    <ProductVersion environment={testCase.environmentName} />
                </MemoryRouter>
            );
            expect(document.querySelector('.environment_name')?.textContent).toContain(testCase.environmentName);
            expect(screen.getByText(testCase.expectedDropDown)).toBeInTheDocument();
        });
    });
});
