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

import { useMemo } from 'react';
import { LocksTable } from '../../components/LocksTable/LocksTable';
import { DisplayLock, searchCustomFilter, sortLocks, useOverview } from '../../utils/store';
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
    const envs = useOverview(({ environments }) => Object.values(environments));
    const envLocks = useMemo(
        () =>
            sortLocks(
                Object.values(envs)
                    .map((env) =>
                        Object.values(env.locks).map(
                            (lock) =>
                                ({
                                    date: lock.createdAt,
                                    environment: env.name,
                                    lockId: lock.lockId,
                                    message: lock.message,
                                    authorName: lock.createdBy?.name,
                                    authorEmail: lock.createdBy?.email,
                                } as DisplayLock)
                        )
                    )
                    .flat(),
                'oldestToNewest'
            ),
        [envs]
    );
    const appLocks = useMemo(
        () =>
            sortLocks(
                Object.values(envs)
                    .map((env) =>
                        Object.values(env.applications)
                            .map((app) =>
                                Object.values(app.locks).map(
                                    (lock) =>
                                        ({
                                            date: lock.createdAt,
                                            environment: env.name,
                                            application: app.name,
                                            lockId: lock.lockId,
                                            message: lock.message,
                                            authorName: lock.createdBy?.name,
                                            authorEmail: lock.createdBy?.email,
                                        } as DisplayLock)
                                )
                            )
                            .flat()
                    )
                    .flat()
                    .filter((lock) => searchCustomFilter(appNameParam, lock.application)),
                'oldestToNewest'
            ),
        [appNameParam, envs]
    );

    return (
        <main className="main-content">
            <LocksTable headerTitle="Environment Locks" columnHeaders={environmentFieldHeaders} locks={envLocks} />
            <LocksTable headerTitle="Application Locks" columnHeaders={applicationFieldHeaders} locks={appLocks} />
        </main>
    );
};
