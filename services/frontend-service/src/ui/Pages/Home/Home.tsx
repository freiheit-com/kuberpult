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
import { ServiceLane } from '../../components/ServiceLane/ServiceLane';
import { useSearchParams } from 'react-router-dom';
import {
    useCurrentlyDeployedAt,
    useFilteredApps,
    useSearchedApplications,
    useReleaseInfo,
    useReleaseDialog,
} from '../../utils/store';
import { ReleaseDialog } from '../../components/ReleaseDialog/ReleaseDialog';

export const Home: React.FC = () => {
    const [params] = useSearchParams();
    const appNameParam = params.get('application') || '';

    const filteredApps = useFilteredApps((params.get('teams') || '').split(',').filter((val) => val !== ''));
    const searchedApp = useSearchedApplications(filteredApps, appNameParam);

    const apps = Object.values(searchedApp);

    const { app, version } = useReleaseDialog(({ app, version }) => ({ app, version }));

    const envs = useCurrentlyDeployedAt(app, version);
    let releaseInfo = useReleaseInfo(app, version);

    if (releaseInfo === undefined) {
        releaseInfo = {};
    }
    return (
        <main className="main-content">
            <ReleaseDialog app={app} version={version} release={releaseInfo} envs={envs}></ReleaseDialog>
            {apps.map((app) => (
                <ServiceLane application={app} key={app.name} />
            ))}
        </main>
    );
};
