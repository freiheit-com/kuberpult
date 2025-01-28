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
import { Home } from '../Pages/Home/Home';
import { EnvironmentsPage } from '../Pages/Environments/EnvironmentsPage';
import { ReleaseHistoryPage } from '../Pages/ReleaseHistory/ReleaseHistoryPage';
import { LocksPage } from '../Pages/Locks/LocksPage';
import { Routes as ReactRoutes, Route, Navigate } from 'react-router-dom';
import { ProductVersionPage } from '../Pages/ProductVersion/ProductVersionPage';
import { CommitInfoPage } from '../Pages/CommitInfo/CommitInfoPage';
import { ReleaseTrainPage } from '../Pages/ReleaseTrain/ReleaseTrainPage';
import { EslWarningsPage } from '../Pages/EslWarnings/EslWarningsPage';
import { CompliancePage } from '../Pages/Compliance/CompliancePage';

const routes = [
    {
        path: `/ui/environments/*`,
        element: <EnvironmentsPage />,
    },
    {
        path: `/ui/environments/:targetEnv/releaseTrain`,
        element: <ReleaseTrainPage />,
    },
    {
        path: `/ui/locks/*`,
        element: <LocksPage />,
    },
    {
        path: `/ui/home/*`,
        element: <Home />,
    },
    {
        path: `/ui/home/releasehistory/:appName`,
        element: <ReleaseHistoryPage />,
    },
    {
        path: `/ui/environments/productVersion/*`,
        element: <ProductVersionPage />,
    },
    {
        path: `/ui/commits/:commit`,
        element: <CommitInfoPage />,
    },
    {
        path: `/ui/commits/`,
        element: <CommitInfoPage />,
    },
    {
        path: `/ui/failedEvents`,
        element: <EslWarningsPage />,
    },
    {
        path: `/ui/compliance`,
        element: <CompliancePage />,
    },
    {
        path: `/*`,
        element: <Navigate replace to="/ui/home" />,
    },
];

export const PageRoutes: React.FC = () => (
    <ReactRoutes>
        {routes.map((route) => (
            <Route key={route.path} path={route.path} element={route.element} />
        ))}
    </ReactRoutes>
);
