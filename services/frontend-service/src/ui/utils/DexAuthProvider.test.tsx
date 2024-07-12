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
import { LoginPage, isTokenValid } from './DexAuthProvider';
import { MemoryRouter } from 'react-router-dom';
import { render } from '@testing-library/react';
import { fakeLoadEverything } from '../../setupTests';

// Mocking document.cookie
Object.defineProperty(document, 'cookie', {
    writable: true,
    value: '',
});

describe('isTokenValid', () => {
    beforeEach(() => {
        // Reset the cookie before each test
        document.cookie = '';
    });

    interface dataEnvT {
        name: string;
        cookie: string;
        expectedTokenValidation: boolean;
    }

    const sampleEnvData: dataEnvT[] = [
        {
            name: 'returns false with expired cookie',
            cookie: 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTd9.6-QS6fw-tEdcmWJP2HNCPzRZaPQgZYwwi5HVoiIX3bo',
            expectedTokenValidation: false,
        },
        {
            name: 'returns false with cookie with no expiring date',
            cookie: 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c',
            expectedTokenValidation: false,
        },
        {
            name: 'returns true with valid cooki',
            cookie: 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTc3Nzd9.p3ApN5elnhhRhrh7DCOF-9suPIXYC36Nycf0nHfxuf8',
            expectedTokenValidation: true,
        },
    ];

    describe.each(sampleEnvData)(`isTokenValid`, (tc) => {
        it(tc.name, () => {
            document.cookie = tc.cookie;
            expect(isTokenValid()).toBe(tc.expectedTokenValidation);
        });
    });
});

describe('LoginPage', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <LoginPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    it('Renders full app', () => {
        fakeLoadEverything(true);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('environment_name')[0]).toHaveTextContent('Log in to Dex');
        expect(
            container.getElementsByClassName('button-main env-card-deploy-btn mdc-button--unelevated')[0]
        ).toHaveTextContent('Login');
    });
});
