import React from 'react';
import { render } from '@testing-library/react';
import { TopAppBar } from './TopAppBar';
import { MemoryRouter } from 'react-router-dom';
import { Console } from 'console';

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
                    <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                </MemoryRouter>
            );
            const message = container.getElementsByClassName('welcome-message');
            tc.cookie === '' ? expect(message).toHaveLength(0) : expect(message[0].textContent).toBe(tc.welcomeMessage);
        });
    });
});
