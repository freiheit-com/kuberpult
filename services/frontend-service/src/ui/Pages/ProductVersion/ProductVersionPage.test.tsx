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
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductVersionPage } from './ProductVersionPage';
import { fakeLoadEverything } from '../../../setupTests';

describe('ProductVersionPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <ProductVersionPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    interface dataEnvT {
        name: string;
        loaded: boolean;
        expectedNumMainContent: number;
        expectedNumSpinner: number;
    }
    const sampleEnvData: dataEnvT[] = [
        {
            name: 'renders main',
            loaded: true,
            expectedNumMainContent: 1,
            expectedNumSpinner: 0,
        },
        {
            name: 'renders spinner',
            loaded: false,
            expectedNumMainContent: 0,
            expectedNumSpinner: 1,
        },
    ];
    describe.each(sampleEnvData)(`Renders ProductVersionPage`, (testcase) => {
        it(testcase.name, () => {
            fakeLoadEverything(testcase.loaded);
            const { container } = getWrapper();
            expect(container.getElementsByClassName('main-content')).toHaveLength(testcase.expectedNumMainContent);
            expect(container.getElementsByClassName('spinner')).toHaveLength(testcase.expectedNumSpinner);
        });
    });
});
