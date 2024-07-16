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
import { ReleasesPage } from './ReleasesPage';
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';

describe('LocksPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <ReleasesPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    interface dataEnvT {
        name: string;
        loaded: boolean;
        enableDex: boolean;
        enableDexValidToken: boolean;
        expectedNumMainContent: number;
        expectedNumSpinner: number;
        expectedNumLoginPage: number;
    }

    const sampleEnvData: dataEnvT[] = [
        {
            name: 'renders main',
            loaded: true,
            enableDex: false,
            enableDexValidToken: false,
            expectedNumMainContent: 1,
            expectedNumSpinner: 0,
            expectedNumLoginPage: 0,
        },
        {
            name: 'renders spinner',
            loaded: false,
            enableDex: false,
            enableDexValidToken: false,
            expectedNumMainContent: 0,
            expectedNumSpinner: 1,
            expectedNumLoginPage: 0,
        },
        {
            name: 'renders Login Page with Dex enabled',
            loaded: true,
            enableDex: true,
            enableDexValidToken: false,
            expectedNumMainContent: 1,
            expectedNumSpinner: 0,
            expectedNumLoginPage: 1,
        },
        {
            name: 'renders page with Dex enabled and valid token',
            loaded: true,
            enableDex: true,
            enableDexValidToken: true,
            expectedNumMainContent: 1,
            expectedNumSpinner: 0,
            expectedNumLoginPage: 0,
        },
    ];

    describe.each(sampleEnvData)(`Renders ReleasesPage`, (testcase) => {
        it(testcase.name, () => {
            fakeLoadEverything(testcase.loaded);
            if (testcase.enableDex) {
                enableDexAuth(testcase.enableDexValidToken);
            }
            const { container } = getWrapper();
            expect(container.getElementsByClassName('main-content')).toHaveLength(testcase.expectedNumMainContent);
            expect(container.getElementsByClassName('spinner')).toHaveLength(testcase.expectedNumSpinner);
            expect(
                container.getElementsByClassName('button-main env-card-deploy-btn mdc-button--unelevated')
            ).toHaveLength(testcase.expectedNumLoginPage);
        });
    });
});
