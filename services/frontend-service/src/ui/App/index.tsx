/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { NavigationBar } from '../components/NavigationBar/NavigationBar';
import { TopAppBar } from '../components/TopAppBar/TopAppBar';
import { PageRoutes } from './PageRoutes';
import '../../assets/app-v2.scss';
import * as React from 'react';
import { PanicOverview, UpdateOverview } from '../utils/store';
import { useApi } from '../utils/GrpcApi';
import {
    AzureAuthProvider,
    UpdateFrontendConfig,
    UpdateReady,
    useAzureAuthSub,
    useReady,
} from '../utils/AzureAuthProvider';

export const App: React.FC = () => {
    const api = useApi;
    const authHeader = useAzureAuthSub(({ authHeader }) => authHeader);
    const authReady = useReady(({ auth }) => auth);

    React.useEffect(() => {
        api.configService()
            .GetConfig({})
            .then(
                (result) => {
                    UpdateFrontendConfig.set(result);
                    UpdateReady.set({ config: 'ready' });
                },
                (error) => {
                    // eslint-disable-next-line no-console
                    alert('Error: Cannot connect to server!\n' + error);
                }
            );
    }, [api]);

    React.useEffect(() => {
        if (authReady === 'ready') {
            const subscription = api
                .overviewService()
                .StreamOverview({}, authHeader)
                .subscribe(
                    (result) => {
                        UpdateOverview.set(result);
                        PanicOverview.set({ error: '' });
                    },
                    (error) => PanicOverview.set({ error: JSON.stringify(error) })
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

    return (
        <AzureAuthProvider>
            <div className={'app-container--v2'}>
                <NavigationBar />
                <div className="mdc-drawer-app-content">
                    <TopAppBar />
                    <PageRoutes />
                </div>
            </div>
        </AzureAuthProvider>
    );
};
