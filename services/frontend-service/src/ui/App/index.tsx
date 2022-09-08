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

export const App: React.FC = () => {
    const api = useApi;
    React.useEffect(() => {
        const subscription = api
            .overviewService()
            .StreamOverview({}) // TODO TE: add auth header
            .subscribe(
                (result) => {
                    UpdateOverview.set(result);
                    PanicOverview.set({});
                },
                (error) => PanicOverview.set(JSON.stringify(error))
            );
        return () => subscription.unsubscribe();
    }, [api]);

    PanicOverview.listen(
        (err) => err,
        (err) => {
            alert('Error: Cannot connect to server!');
        }
    );

    return (
        <div className={'app-container--v2'}>
            <NavigationBar />
            <div className="mdc-drawer-app-content">
                <TopAppBar />
                <PageRoutes />
            </div>
        </div>
    );
};
