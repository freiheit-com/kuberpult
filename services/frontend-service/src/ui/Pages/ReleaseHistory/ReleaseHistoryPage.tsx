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
import { getAppDetails, useAppDetailsForApp } from '../../utils/store';
import React, { useEffect } from 'react';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { Spinner } from '../../components/Spinner/Spinner';

export const ReleaseHistoryPage: React.FC = () => {
    const url = window.location.pathname.split('/');
    const app_name = url[url.length - 1];
    const appDetails = useAppDetailsForApp(app_name);
    const { authHeader } = useAzureAuthSub((auth) => auth);

    useEffect(() => {
        getAppDetails(app_name, authHeader);
    }, [app_name, authHeader]);

    if (!appDetails) {
        return <Spinner message={'Loading History...'} />;
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
