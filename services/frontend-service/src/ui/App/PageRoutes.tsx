import { Home } from '../Pages/Home/Home';
import { EnvironmentsPage } from '../Pages/Environments/EnvironmentsPage';
import { ReleasesPage } from '../Pages/Releases/ReleasesPage';
import { LocksPage } from '../Pages/Locks/LocksPage';
import { Routes as ReactRoutes, Route, Navigate } from 'react-router-dom';

const routes = [
    {
        path: `/environments/*`,
        element: <EnvironmentsPage />,
    },
    {
        path: `/locks/*`,
        element: <LocksPage />,
    },
    {
        path: `/home/*`,
        element: <Home />,
    },
    {
        path: `/home/releases/:appName`,
        element: <ReleasesPage />,
    },
    {
        path: `/*`,
        element: <Navigate replace to="/v2/home" />,
    },
];

export const PageRoutes: React.FC = () => (
    <ReactRoutes>
        {routes.map((route) => (
            <Route key={route.path} path={route.path} element={route.element} />
        ))}
    </ReactRoutes>
);
