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
