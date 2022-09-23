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
import { createStore } from 'react-use-sub';
import { DisplayLock, GetOverviewResponse } from '../../api/api';

const emptyOverview: GetOverviewResponse = { applications: {}, environments: {} };
export const [useOverview, UpdateOverview] = createStore(emptyOverview);

export const [_, PanicOverview] = createStore({ error: '' });

// returns all application names
export const useApplicationNames = () =>
    useOverview(({ applications }) => Object.keys(applications).sort((a, b) => a.localeCompare(b)));

// return all environment locks
export const useEnvironmentLocks = () =>
    useOverview(({ environments }) => {
        const locks = Object.values(environments)
            .map((environment) =>
                Object.values(environment.locks).map(
                    (lockInfo) =>
                        ({
                            date: lockInfo.createdAt,
                            environment: environment.name,
                            lockId: lockInfo.lockId,
                            message: lockInfo.message,
                            authorName: lockInfo.createdBy?.name,
                            authorEmail: lockInfo.createdBy?.email,
                        } as DisplayLock)
                )
            )
            .filter((displayLock) => displayLock.length !== 0)
            .flat();
        return sortLocks(locks);
    });

// return all applications locks
export const useApplicationLocks = () =>
    useOverview(({ environments }) => {
        const locks = Object.values(environments)
            .map((environment) =>
                Object.values(environment.applications)
                    .map((application) =>
                        Object.values(application.locks).map(
                            (lockInfo) =>
                                ({
                                    date: lockInfo.createdAt,
                                    application: application.name,
                                    environment: environment.name,
                                    lockId: lockInfo.lockId,
                                    message: lockInfo.message,
                                    authorName: lockInfo.createdBy?.name,
                                    authorEmail: lockInfo.createdBy?.email,
                                } as DisplayLock)
                        )
                    )
                    .filter((displayLock) => displayLock.length !== 0)
            )
            .filter((displayLock) => displayLock.length !== 0)
            .map((displayLock) => displayLock[0])
            .flat();
        return sortLocks(locks);
    });

const sortLocks = (displayLocks: DisplayLock[]) =>
    displayLocks.sort((a: DisplayLock, b: DisplayLock) => {
        if (a.date === b.date) {
            if (a.environment === b.environment) {
                if (a.application !== undefined && b.application !== undefined) {
                    if (a.application === b.application) {
                        return a.lockId < b.lockId ? -1 : a.lockId === b.lockId ? 0 : 1;
                    }
                    return a.application < b.application ? -1 : 1;
                }
                return a.lockId < b.lockId ? -1 : a.lockId === b.lockId ? 0 : 1;
            }
            return a.environment < b.environment ? -1 : 1;
        }
        return a.date > b.date ? -1 : 1;
    });

// returns the release number {$version} of {$application}
export const useRelease = (application: string, version: number) =>
    useOverview(
        ({ applications }) =>
            applications[application].releases.find((r) =>
                version === -1 ? r.undeployVersion : r.version === version
            )!
    );

// returns the release versions that are currently deployed to at least one environment
export const useDeployedReleases = (application: string) =>
    useOverview(({ environments }) =>
        [
            ...new Set(
                Object.values(environments)
                    .filter((env) => env.applications[application])
                    .map((env) =>
                        env.applications[application].undeployVersion ? -1 : env.applications[application].version
                    )
            ),
        ].sort((a, b) => (a === -1 ? -1 : b === -1 ? 1 : b - a))
    );

// returns the environments where a release is currently deployed
export const useCurrentlyDeployedAt = (application: string, version: number) =>
    useOverview(({ environments }) =>
        Object.values(environments)
            .filter(
                (env) =>
                    env.applications[application] &&
                    (version === -1
                        ? env.applications[application].undeployVersion
                        : env.applications[application].version === version)
            )
            .map((e) => e.name)
    );
