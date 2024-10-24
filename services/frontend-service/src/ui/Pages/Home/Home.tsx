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
import { ServiceLane } from '../../components/ServiceLane/ServiceLane';
import { useSearchParams } from 'react-router-dom';
import { useAppDetails, useApplicationsFilteredAndSorted, useGlobalLoadingState } from '../../utils/store';
import React from 'react';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { hideWithoutWarnings, hideMinors } from '../../utils/Links';

export const Home: React.FC = () => {
    const [params] = useSearchParams();
    const appNameParam = params.get('application') || '';
    const teamsParam = (params.get('teams') || '').split(',').filter((val) => val !== '');

    const searchedApp = useApplicationsFilteredAndSorted(teamsParam, hideWithoutWarnings(params), appNameParam);
    const allAppDetails = useAppDetails((m) => m);
    const apps = Object.values(searchedApp);
    const element = useGlobalLoadingState();
    if (element) {
        return element;
    }

    return (
        <div>
            <TopAppBar showAppFilter={true} showTeamFilter={true} showWarningFilter={true} />
            <main className="main-content">
                {apps.map((app) => (
                    <ServiceLane
                        application={app}
                        hideMinors={hideMinors(params)}
                        allAppDetails={allAppDetails}
                        key={app.name}
                    />
                ))}
            </main>
        </div>
    );
};
