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

import { EnvLocksTable } from '../../components/LocksTable/EnvLocksTable';
import { AppLocksTable } from '../../components/LocksTable/AppLocksTable';
import {
    useEnvironmentLockIDs,
    // useEnvironmentLocks,
    useFilteredApplicationLockIDs,
    // useFilteredApplicationLocks,
} from '../../utils/store';
import { useSearchParams } from 'react-router-dom';

const applicationFieldHeaders = [
    'Date',
    'Environment',
    'Application',
    'Lock Id',
    'Message',
    'Author Name',
    'Author Email',
    '',
];

const environmentFieldHeaders = ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''];

export const LocksPage: React.FC = () => {
    const [params] = useSearchParams();
    const appNameParam = params.get('application');

    // const appLocks = useFilteredApplicationLocks(appNameParam);
    // const envLocks = useEnvironmentLocks();
    const appLocks = useFilteredApplicationLockIDs(appNameParam || '');
    const envLocks = useEnvironmentLockIDs();

    return (
        <main className="main-content">
            <EnvLocksTable headerTitle="Environment Locks" columnHeaders={environmentFieldHeaders} lockIDs={envLocks} />
            <AppLocksTable headerTitle="Application Locks" columnHeaders={applicationFieldHeaders} lockIDs={appLocks} />
        </main>
    );
};
