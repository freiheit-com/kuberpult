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
