import { NavigationBar } from '../components/NavigationBar/NavigationBar';
import { TopAppBar } from '../components/TopAppBar/TopAppBar';
import { ReleaseDialog } from '../components/ReleaseDialog/ReleaseDialog';
import { PageRoutes } from './PageRoutes';
import '../../assets/app-v2.scss';
import * as React from 'react';
import { PanicOverview, UpdateOverview, useReleaseDialog, useAllDeployedAt, useReleaseInfo } from '../utils/store';
import { useApi } from '../utils/GrpcApi';
import { AzureAuthProvider, UpdateFrontendConfig, useAzureAuthSub } from '../utils/AzureAuthProvider';

export const App: React.FC = () => {
    const api = useApi;
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        api.configService()
            .GetConfig({})
            .then(
                (result) => {
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
                .subscribe(
                    (result) => {
                        UpdateOverview.set(result);
                        PanicOverview.set({ error: '' });
                    },
                    (error) => PanicOverview.set({ error: JSON.stringify({ msg: 'error in streamoverview', error }) })
                );
            return () => subscription.unsubscribe();
        }
    }, [api, authHeader, authReady]);

    PanicOverview.listen(
        (err) => err.error,
        (err) => {
            // eslint-disable-next-line no-console
            console.log('Error: Cannot connect to server!\n' + err);
        }
    );

    const { app, version } = useReleaseDialog(({ app, version }) => ({ app, version }));
    const envs = useAllDeployedAt(app);
    const releaseInfo = useReleaseInfo(app, version);

    return (
        <AzureAuthProvider>
            <div className={'app-container--v2'}>
                <ReleaseDialog app={app} version={version} release={releaseInfo} envs={envs} />
                <NavigationBar />
                <div className="mdc-drawer-app-content">
                    <TopAppBar />
                    <PageRoutes />
                </div>
            </div>
        </AzureAuthProvider>
    );
};
