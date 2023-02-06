import { Releases } from '../../components/Releases/Releases';

export const ReleasesPage: React.FC = () => {
    const url = window.location.href.split('/');
    const app_name = url[url.length - 1];
    return (
        <main className="main-content">
            <Releases app={app_name} />
        </main>
    );
};
