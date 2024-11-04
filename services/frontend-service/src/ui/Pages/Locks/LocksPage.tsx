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
import React, { useMemo } from 'react';
import { LocksTable } from '../../components/LocksTable/LocksTable';
import {
    searchCustomFilter,
    sortLocks,
    useApplications,
    useEnvironments,
    useGlobalLoadingState,
    useTeamLocks,
} from '../../utils/store';
import { useSearchParams } from 'react-router-dom';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';

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

const teamFieldHeaders = ['Date', 'Environment', 'Team', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''];

const environmentFieldHeaders = ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''];
export const LocksPage: React.FC = () => {
    const [params] = useSearchParams();
    const appNameParam = params.get('application');
    const envs = useEnvironments();
    const allApps = useApplications();
    let teamLocks = useTeamLocks(allApps);
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

    teamLocks = useMemo(() => sortLocks(teamLocks, 'oldestToNewest'), [teamLocks]);

    //Goes through all envs and all apps and checks for locks for each app
    const appLocks = useMemo(
        () =>
            sortLocks(
                Object.values(envs)
                    .map((env) =>
                        allApps
                            .map((app) =>
                                env.appLocks[app.name]
                                    ? env.appLocks[app.name].locks.map((lock) => ({
                                          date: lock.createdAt,
                                          environment: env.name,
                                          application: app.name,
                                          lockId: lock.lockId,
                                          message: lock.message,
                                          authorName: lock.createdBy?.name,
                                          authorEmail: lock.createdBy?.email,
                                      }))
                                    : []
                            )
                            .flat()
                    )
                    .flat()
                    .filter((lock) => searchCustomFilter(appNameParam, lock.application)),
                'oldestToNewest'
            ),
        [appNameParam, envs, allApps]
    );

    const element = useGlobalLoadingState();
    if (element) {
        return element;
    }
    return (
        <div>
            <TopAppBar showAppFilter={true} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content">
                <LocksTable headerTitle="Environment Locks" columnHeaders={environmentFieldHeaders} locks={envLocks} />
                <LocksTable headerTitle="Application Locks" columnHeaders={applicationFieldHeaders} locks={appLocks} />
                <LocksTable headerTitle="Team Locks" columnHeaders={teamFieldHeaders} locks={teamLocks} />
            </main>
        </div>
    );
};
