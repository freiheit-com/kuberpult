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
import { GetOverviewResponse } from '../../api/api';

export interface DisplayLock {
    date: Date;
    environment: string;
    application?: string;
    message: string;
    lockId: string;
    authorName: string;
    authorEmail: string;
}

const emptyOverview: GetOverviewResponse = { applications: {}, environments: {} };
export const [useOverview, UpdateOverview] = createStore(emptyOverview);

export const [_, PanicOverview] = createStore({ error: '' });

// returns all application names
export const useApplicationNames = () =>
    useOverview(({ applications }) => Object.keys(applications).sort((a, b) => a.localeCompare(b)));

// return all environment locks
export const useEnvironmentLocks = () =>
    useOverview(({ environments }) =>
        Object.values(
            Object.values(environments)
                .map((environment) => environment.locks)
                .reduce((acc, val) => ({ ...acc, ...val }))
        )
            .sort((a, b) => {
                if (!a.createdAt) {
                    return b.createdAt ? 1 : 0;
                }
                return b.createdAt ? a.createdAt.valueOf() - b.createdAt.valueOf() : -1;
            })
            .map((v) => v.lockId)
    );

export const useEnvironmentLock = (id: string) => ({
    ...useOverview(
        ({ environments }) =>
            Object.values(
                Object.values(environments)
                    .map((environment) => environment.locks)
                    .reduce((acc, val) => ({ ...acc, ...val }))
            ).find((lock) => lock.lockId === id)!
    ),
    environment: useOverview(({ environments }) =>
        Object.values(environments).find((environment) => environment.locks[id])
    )!.name,
});

export const sortLocks = (displayLocks: DisplayLock[], sorting: string) => {
    const sortMethod = sorting === 'descending' ? -1 : 1;
    displayLocks.sort((a: DisplayLock, b: DisplayLock) => {
        const aValues: (Date | string)[] = [];
        const bValues: (Date | string)[] = [];
        Object.values(a).forEach((val) => aValues.push(val));
        Object.values(b).forEach((val) => bValues.push(val));
        for (let i = 0; i < aValues.length; i++) {
            if (aValues[i] < bValues[i]) {
                if (aValues[i] instanceof Date) return -sortMethod;
                return sortMethod;
            } else if (aValues[i] > bValues[i]) {
                if (aValues[i] instanceof Date) return sortMethod;
                return -sortMethod;
            }
            if (aValues[aValues.length - 1] === bValues[aValues.length - 1]) {
                return 0;
            }
        }
        return 0;
    });
    return displayLocks;
};

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
