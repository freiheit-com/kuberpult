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
import { Releases } from '../../components/Releases/Releases';
import { useGlobalLoadingState } from '../../utils/store';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import React from 'react';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';

export const ReleasesPage: React.FC = () => {
    const url = window.location.pathname.split('/');
    const app_name = url[url.length - 1];

    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content">
                <Releases app={app_name} />
            </main>
        </div>
    );
};
