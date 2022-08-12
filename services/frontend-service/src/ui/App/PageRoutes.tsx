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
import { Home } from '../Pages/Home/Home';
import { EnvironmentsPage } from '../Pages/Environments/EnvironmentsPage';
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
