import { App as LegacyApp } from '../ui/App';
import { App } from './App';
import { Routes as ReactRoutes, Route } from 'react-router-dom';

const prefix = 'v2';
const routes = [
    {
        path: `/${prefix}`,
        element: <App />,
    },
    {
        // If none of the above paths are matched, then this route is chosen
        path: '*',
        element: <LegacyApp />,
    },
];

export const Routes: React.FC = () => (
    <ReactRoutes>
        {routes.map((route) => (
            <Route key={route.path} path={route.path} element={route.element} />
        ))}
    </ReactRoutes>
);
