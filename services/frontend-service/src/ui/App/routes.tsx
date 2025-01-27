import { Navigate } from 'react-router-dom';
import { CommitInfoPage } from '../Pages/CommitInfo/CommitInfoPage';
import { EnvironmentsPage } from '../Pages/Environments/EnvironmentsPage';
import { EslWarningsPage } from '../Pages/EslWarnings/EslWarningsPage';
import { Home } from '../Pages/Home/Home';
import { LocksPage } from '../Pages/Locks/LocksPage';
import { ProductVersionPage } from '../Pages/ProductVersion/ProductVersionPage';
import { ReleaseHistoryPage } from '../Pages/ReleaseHistory/ReleaseHistoryPage';
import { ReleaseTrainPage } from '../Pages/ReleaseTrain/ReleaseTrainPage';
import { CompliancePage } from '../Pages/Compliance/CompliancePage';

export const routes = [
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
