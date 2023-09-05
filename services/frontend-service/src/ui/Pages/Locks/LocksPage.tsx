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
import React, { useMemo } from 'react';
import { LocksTable } from '../../components/LocksTable/LocksTable';
import { searchCustomFilter, sortLocks, useEnvironments, useGlobalLoadingState } from '../../utils/store';
import { useSearchParams } from 'react-router-dom';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';

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
    const envs = useEnvironments();
    const envLocks = useMemo(
        () =>
            sortLocks(
                Object.values(envs)
                    .map((env) =>
                        Object.values(env.locks).map((lock) => ({
                            date: lock.createdAt,
                            environment: env.name,
                            lockId: lock.lockId,
                            message: lock.message,
                            authorName: lock.createdBy?.name,
                            authorEmail: lock.createdBy?.email,
                        }))
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
                                Object.values(app.locks).map((lock) => ({
                                    date: lock.createdAt,
                                    environment: env.name,
                                    application: app.name,
                                    lockId: lock.lockId,
                                    message: lock.message,
                                    authorName: lock.createdBy?.name,
                                    authorEmail: lock.createdBy?.email,
                                }))
                            )
                            .flat()
                    )
                    .flat()
                    .filter((lock) => searchCustomFilter(appNameParam, lock.application)),
                'oldestToNewest'
            ),
        [appNameParam, envs]
    );
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }
    return (
        <main className="main-content">
            <LocksTable headerTitle="Environment Locks" columnHeaders={environmentFieldHeaders} locks={envLocks} />
            <LocksTable headerTitle="Application Locks" columnHeaders={applicationFieldHeaders} locks={appLocks} />
        </main>
    );
};
