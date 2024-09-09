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
import { App } from './index';
import { render } from '@testing-library/react';
import { Spy } from 'spy4js';
import { AzureAuthSub } from '../utils/AzureAuthProvider';
import { Observable } from 'rxjs';
import { UpdateOverview } from '../utils/store';
import { MemoryRouter } from 'react-router-dom';

Spy.mockModule('../components/NavigationBar/NavigationBar', 'NavigationBar');
Spy.mockModule('../components/TopAppBar/TopAppBar', 'TopAppBar');
Spy.mockModule('../components/ReleaseDialog/ReleaseDialog', 'ReleaseDialog');
Spy.mockModule('./PageRoutes', 'PageRoutes');
Spy.mockModule('../components/snackbar/snackbar', 'Snackbar');
Spy.mockModule('../utils/AzureAuthProvider', 'AzureAuthProvider');

const mock_GetConfig = Spy('Config');
const mock_StreamOverview = Spy('Overview');
const mock_StreamStatus = Spy('Status');

jest.mock('../utils/GrpcApi', () => ({
    // useApi is a constant, so we mock it by mocking the module and spying on a getter method instead
    get useApi() {
        return {
            configService: () => ({
                GetConfig: () => mock_GetConfig(),
            }),
            overviewService: () => ({
                StreamOverview: () => mock_StreamOverview(),
            }),
            rolloutService: () => ({
                StreamStatus: () => mock_StreamStatus(),
            }),
        };
    },
}));

const getNode = (): JSX.Element => (
    <MemoryRouter>
        <App />
    </MemoryRouter>
);
const getWrapper = () => render(getNode());

describe('App uses the API', () => {
    beforeAll(() => {
        jest.useFakeTimers();
    });

    afterAll(() => {
        jest.runOnlyPendingTimers();
        jest.useRealTimers();
    });

    it('subscribes to StreamOverview', () => {
        // given
        mock_StreamOverview.returns(
            new Observable((observer) => {
                observer.next({ applications: 'test-application' });
            })
        );
        mock_GetConfig.returns(Promise.resolve('test-config'));
        AzureAuthSub.set({ authReady: true });
        mock_StreamStatus.returns(
            new Observable((observer) => {
                observer.next({});
            })
        );

        // when
        getWrapper();

        // then
        expect(UpdateOverview.get().applications).toBe('test-application');
    });

    it('retries subscription to StreamOverview on Error', () => {
        // given
        let subscriptionCount = 0;
        mock_StreamOverview.returns(
            new Observable((observer) => {
                observer.error('error');
                subscriptionCount++;
            })
        );
        mock_StreamStatus.returns(
            new Observable((observer) => {
                observer.next({});
            })
        );
        mock_GetConfig.returns(Promise.resolve('test-config'));
        AzureAuthSub.set({ authReady: true });

        // when
        getWrapper();

        // when
        jest.advanceTimersByTime(5000);
        // then - 4 retries in 5s
        expect(subscriptionCount).toBe(4);
        // when
        jest.advanceTimersByTime(5000);
        // then - 6 retries in 10s
        expect(subscriptionCount).toBe(6);
        // when
        jest.advanceTimersByTime(6000);
        // then - 7 retries in 16s
        expect(subscriptionCount).toBe(7);

        // when - advance time by 1 minute
        jest.advanceTimersByTime(1 * 60 * 1000);
        // then - after one minute and 16 seconds, there should be 12 retries + first attempt
        expect(subscriptionCount).toBe(13);
    });
});
