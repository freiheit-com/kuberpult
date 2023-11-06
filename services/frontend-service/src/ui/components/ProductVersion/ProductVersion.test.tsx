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
import { MemoryRouter } from 'react-router-dom';
import { ProductVersion } from './ProductVersion';
import { render } from '@testing-library/react';

describe('Product Version Data', () => {
    type TestData = {
        name: string;
        environmentName: string;
    };
    const data: TestData[] = [
        {
            name: 'No tags to Display',
            environmentName: 'tester',
        },
    ];

    describe.each(data)(`Displays Product Version Page`, (testCase) => {
        it(testCase.name, () => {
            render(
                <MemoryRouter>
                    <ProductVersion environment={testCase.environmentName} />
                </MemoryRouter>
            );
            expect(document.querySelector('.environment_name')?.textContent).toContain(testCase.environmentName);
        });
    });
});
