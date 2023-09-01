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
import { NavigationBar } from '../components/NavigationBar/NavigationBar';
import { TopAppBar } from '../components/TopAppBar/TopAppBar';
import { ReleaseDialog } from '../components/ReleaseDialog/ReleaseDialog';
import { PageRoutes } from './PageRoutes';
import '../../assets/app-v2.scss';
import * as React from 'react';
import {
    FlushRolloutStatus,
    PanicOverview,
    showSnackbarWarn,
    UpdateFrontendConfig,
    UpdateOverview,
    UpdateRolloutStatus,
    useKuberpultVersion,
    useReleaseDialogParams,
} from '../utils/store';
import { useApi } from '../utils/GrpcApi';
import { AzureAuthProvider, useAzureAuthSub } from '../utils/AzureAuthProvider';
import { Snackbar } from '../components/snackbar/snackbar';
import { mergeMap, retryWhen } from 'rxjs/operators';
import { Observable, throwError, timer } from 'rxjs';
import { GetFrontendConfigResponse } from '../../api/api';

// retry strategy: retries the observable subscription with randomized exponential backoff
// source: https://www.learnrxjs.io/learn-rxjs/operators/error_handling/retrywhen#examples
function retryStrategy(maxRetryAttempts: number) {
    return (attempts: Observable<any>): Observable<any> =>
        attempts.pipe(
            mergeMap((error, retryAttempt) => {
                if (retryAttempt >= maxRetryAttempts) {
                    return throwError(error);
                }
                // backoff time in seconds = 2^attempt number (exponential) + random
                const backoffTime = 1000 * (2 ** retryAttempt + Math.random());
                return timer(backoffTime);
            })
        );
}

export const App: React.FC = () => {
    const api = useApi;
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);

    const kuberpultVersion = useKuberpultVersion();
    React.useEffect(() => {
        if (kuberpultVersion !== '') {
            document.title = 'Kuberpult v' + kuberpultVersion;
        }
    }, [kuberpultVersion, api]);

    React.useEffect(() => {
        api.configService()
            .GetConfig({}) // the config service does not require authorisation
            .then(
                (result: GetFrontendConfigResponse) => {
                    UpdateFrontendConfig.set({ configs: result, configReady: true });
                },
                (error) => {
                    // eslint-disable-next-line no-console
                    console.log('Error: Cannot connect to server!\n' + error);
                }
            );
    }, [api]);

    React.useEffect(() => {
        if (authReady) {
            const subscription = api
                .overviewService()
                .StreamOverview({}, authHeader)
                .pipe(retryWhen(retryStrategy(8)))
                .subscribe(
                    (result) => {
                        UpdateOverview.set(result);
                        UpdateOverview.set({ loaded: true });
                        PanicOverview.set({ error: '' });
                    },
                    (error) => {
                        PanicOverview.set({ error: JSON.stringify({ msg: 'error in streamoverview', error }) });
                        showSnackbarWarn('Connection Error: Refresh the page');
                    }
                );
            return (): void => subscription.unsubscribe();
        }
    }, [api, authHeader, authReady]);

    React.useEffect(() => {
        if (authReady) {
            const subscription = api
                .rolloutService()
                .StreamStatus({}, authHeader)
                .pipe(retryWhen(retryStrategy(8)))
                .subscribe(
                    (result) => {
                        UpdateRolloutStatus(result);
                    },
                    (error) => {
                        PanicOverview.set({ error: JSON.stringify({ msg: 'error in rolloutstatus', error }) });
                        FlushRolloutStatus();
                    }
                );
            return (): void => subscription.unsubscribe();
        }
    }, [api, authHeader, authReady]);

    PanicOverview.listen(
        (err) => err.error,
        (err) => {
            // eslint-disable-next-line no-console
            console.log('Error: Cannot connect to server!\n' + err);
        }
    );

    const { app, version } = useReleaseDialogParams();

    return (
        <AzureAuthProvider>
            <div className={'app-container--v2'}>
                {app && version ? <ReleaseDialog app={app} version={version} /> : null}
                <NavigationBar />
                <div className="mdc-drawer-app-content">
                    <TopAppBar />
                    <PageRoutes />
                    <Snackbar />
                </div>
            </div>
        </AzureAuthProvider>
    );
};
