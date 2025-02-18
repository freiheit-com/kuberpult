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
import React from 'react';
import { render } from '@testing-library/react';
import { TopAppBar } from './TopAppBar';
import { MemoryRouter } from 'react-router-dom';

describe('TopAppBar', () => {
    beforeEach(() => {
        document.cookie = '';
    });
    interface dataEnvT {
        name: string;
        cookie: string;
        welcomeMessage: string;
    }
    const sampleEnvData: dataEnvT[] = [
        {
            name: 'no cookie',
            cookie: '',
            welcomeMessage: '',
        },
        {
            name: 'cookie defined with no user email',
            cookie: 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE3MjA2MjE5OTc3Nzd9.p3ApN5elnhhRhrh7DCOF-9suPIXYC36Nycf0nHfxuf8',
            welcomeMessage: 'Welcome, Guest!',
        },
        {
            name: 'cookie defined with a user email',
            cookie: 'kuberpult.oauth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJlbWFpbCI6InRlc3QudGVzdEB0ZXN0LmNvbSJ9.wgoizCsQb6AuFsbINF5OHfmfipdj5MuEUwJjZaqsBOg',
            welcomeMessage: 'Welcome, test.test@test.com!',
        },
    ];
    describe.each(sampleEnvData)(`welcomeMessage`, (tc) => {
        it(tc.name, () => {
            document.cookie = tc.cookie;
            const { container } = render(
                <MemoryRouter>
                    <TopAppBar
                        showAppFilter={false}
                        showTeamFilter={false}
                        showWarningFilter={false}
                        showGitSyncStatus={false}
                    />
                </MemoryRouter>
            );
            const message = container.getElementsByClassName('welcome-message');
            tc.cookie === '' ? expect(message).toHaveLength(0) : expect(message[0].textContent).toBe(tc.welcomeMessage);
        });
    });
});
