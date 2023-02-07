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
import { ServiceLane } from '../../components/ServiceLane/ServiceLane';
import { useSearchParams } from 'react-router-dom';
import { useFilteredApps, useSearchedApplications } from '../../utils/store';

export const Home: React.FC = () => {
    const [params] = useSearchParams();
    const appNameParam = params.get('application') || '';

    const filteredApps = useFilteredApps((params.get('teams') || '').split(',').filter((val) => val !== ''));
    const searchedApp = useSearchedApplications(filteredApps, appNameParam);

    const apps = Object.values(searchedApp);

    return (
        <main className="main-content">
            {apps.map((app) => (
                <ServiceLane application={app} key={app.name} />
            ))}
        </main>
    );
};
