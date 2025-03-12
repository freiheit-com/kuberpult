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
import { NavigationBar } from '../components/NavigationBar/NavigationBar';
import { ReleaseDialog } from '../components/ReleaseDialog/ReleaseDialog';
import { PageRoutes } from './PageRoutes';
import '../../assets/app-v2.scss';
import * as React from 'react';
import {
    AppDetailsState,
    emptyAppLocks,
    EnableGitSyncStatus,
    EnableRolloutStatus,
    FlushGitSyncStatus,
    FlushRolloutStatus,
    PanicOverview,
    showSnackbarWarn,
    UpdateAllApplicationLocks,
    updateAllEnvLocks,
    updateAppDetails,
    UpdateFrontendConfig,
    UpdateGitSyncStatus,
    UpdateOverview,
    UpdateRolloutStatus,
    useKuberpultVersion,
    useReleaseDialogParams,
} from '../utils/store';
import { useApi } from '../utils/GrpcApi';
import { AzureAuthProvider, useAzureAuthSub } from '../utils/AzureAuthProvider';
import { Snackbar } from '../components/snackbar/snackbar';
import { mergeMap, retryWhen } from 'rxjs/operators';
import { Observable, timer } from 'rxjs';
import { AllAppLocks, GetFrontendConfigResponse } from '../../api/api';
import { EnvironmentConfigDialog } from '../components/EnvironmentConfigDialog/EnvironmentConfigDialog';
import { getOpenEnvironmentConfigDialog } from '../utils/Links';
import { useSearchParams } from 'react-router-dom';
import { TooltipProvider } from '../components/tooltip/tooltip';

// retry strategy: retries the observable subscription with a linear backoff
// source: https://www.learnrxjs.io/learn-rxjs/operators/error_handling/retrywhen#examples
function retryStrategy(maxWaitTimeMinutes: number) {
    return (attempts: Observable<any>): Observable<any> =>
        attempts.pipe(
            mergeMap((error, retryAttempt) => {
                if (error.code === 12) {
                    // Error code 12 means "not implemented". That is what we get when the rollout service is not enabled
                    // so we don't want to retry in this case
                    throw error;
                }

                // retry forever with a maximum wait time of maxWaitTimeMinutes minutes
                const maxWaitTimeSeconds = maxWaitTimeMinutes * 60;
                if (retryAttempt >= maxWaitTimeSeconds) {
                    return timer(maxWaitTimeSeconds * 1000);
                } else {
                    return timer(retryAttempt * 1000);
                }
            })
        );
}

export const App: React.FC = () => {
    const api = useApi;
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);

    const kuberpultVersion = useKuberpultVersion();
    React.useEffect(() => {
        if (kuberpultVersion !== '') {
            document.title = 'Kuberpult ' + kuberpultVersion;
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
                .gitService()
                .StreamGitSyncStatus({}, authHeader)
                .pipe(retryWhen(retryStrategy(1)))
                .subscribe(
                    (result) => {
                        UpdateGitSyncStatus(result);
                    },
                    (error) => {
                        if (error.code === 12) {
                            // Error code 12 means "not implemented". That is what we get when the rollout service is not enabled.
                            FlushGitSyncStatus();
                            return;
                        }
                        PanicOverview.set({
                            error: JSON.stringify({ msg: 'error in StreamGitSyncStatus', error }),
                        });
                        showSnackbarWarn('Connection Error: Refresh the page');
                        EnableGitSyncStatus();
                    }
                );
            return (): void => subscription.unsubscribe();
        }
    }, [api, authHeader, authReady]);

    React.useEffect(() => {
        if (authReady) {
            const subscription = api
                .overviewService()
                .StreamOverview({}, authHeader)
                .pipe(retryWhen(retryStrategy(1)))
                .subscribe(
                    (result) => {
                        UpdateOverview.set(result);
                        UpdateOverview.set({ loaded: true });
                        PanicOverview.set({ error: '' });

                        // When there's an update of the overview
                        // we keep the app details that we have,
                        // and add new ones for the apps that we don't know yet:
                        const details = updateAppDetails.get();
                        result.lightweightApps?.forEach((elem) => {
                            if (!details[elem.name]) {
                                details[elem.name] = {
                                    appDetailState: AppDetailsState.NOTREQUESTED,
                                    details: undefined,
                                    updatedAt: undefined,
                                    errorMessage: '',
                                };
                            }
                        });
                        updateAppDetails.set(details);
                        // Get App Locks
                        api.overviewService()
                            .GetAllAppLocks({}, authHeader)
                            .then((res) => {
                                UpdateAllApplicationLocks.set(res);
                            })
                            .catch((e) => {
                                PanicOverview.set({ error: JSON.stringify({ msg: 'error in GetAllAppLocks', e }) });
                            });
                        // Get Env Locks
                        api.overviewService()
                            .GetAllEnvTeamLocks({}, authHeader)
                            .then((res) => {
                                updateAllEnvLocks.set(res);
                            })
                            .catch((e) => {
                                PanicOverview.set({ error: JSON.stringify({ msg: 'error in GetAllEnvTeamLocks', e }) });
                            });
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
                .pipe(retryWhen(retryStrategy(1)))
                .subscribe(
                    (result) => {
                        UpdateRolloutStatus(result);
                    },
                    (error) => {
                        if (error.code === 12) {
                            // Error code 12 means "not implemented". That is what we get when the rollout service is not enabled.
                            FlushRolloutStatus();
                            return;
                        }
                        PanicOverview.set({ error: JSON.stringify({ msg: 'error in rolloutstatus', error }) });
                        EnableRolloutStatus();
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

    const [params] = useSearchParams();
    const { app, version } = useReleaseDialogParams();
    const currentOpenConfig = getOpenEnvironmentConfigDialog(params);

    return (
        <AzureAuthProvider>
            <div className={'app-container--v2'}>
                {app && version ? <ReleaseDialog app={app} version={version} /> : null}
                {currentOpenConfig.length > 0 ? <EnvironmentConfigDialog environmentName={currentOpenConfig} /> : null}
                <NavigationBar />
                <div className="mdc-drawer-app-content">
                    <PageRoutes />
                    <Snackbar />
                </div>
                <TooltipProvider />
            </div>
        </AzureAuthProvider>
    );
};
