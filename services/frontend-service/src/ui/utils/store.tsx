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
import { Application, BatchRequest, GetOverviewResponse, BatchAction } from '../../api/api';

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

const emptyBatch: BatchRequest = { actions: [] };
export const [useAction, UpdateAction] = createStore(emptyBatch);

export const [_, PanicOverview] = createStore({ error: '' });

export const [useReleaseDialog, UpdateReleaseDialog] = createStore({ app: '', version: 0 });

export const useActions = () => useAction(({ actions }) => actions);

export const updateActions = (actions: BatchAction[]) => UpdateAction.set({ actions: actions });

export const updateReleaseDialog = (_open: boolean, app: string, version: number) => {
    UpdateReleaseDialog.set({ app: app, version: version });
};
export const addAction = (action: BatchAction) =>
    UpdateAction.set({ actions: [...UpdateAction.get().actions, action] });

export const deleteAction = (action: BatchAction) =>
    UpdateAction.set(({ actions }) => ({
        // create comparison function
        actions: actions.filter((act) => JSON.stringify(act).localeCompare(JSON.stringify(action))),
    }));

// returns all application names
// doesn't return empty team names (i.e.: '')
// doesn't return repeated team names
export const useTeamNames = () =>
    useOverview(({ applications }) => [
        ...new Set(
            Object.values(applications)
                .map((app: Application) => app.team.trim() || '<No Team>')
                .sort((a, b) => a.localeCompare(b))
        ),
    ]);

// returns applications filtered by dropdown and sorted by team name and then by app name
export const useFilteredApps = (teams: string[]) =>
    useOverview(({ applications }) =>
        Object.values(applications).filter(
            (app) => teams.length === 0 || teams.includes(app.team.trim() || '<No Team>')
        )
    );

// returns all environment names
export const useEnvironmentNames = () =>
    useOverview(({ environments }) => Object.keys(environments).sort((a, b) => a.localeCompare(b)));

// returns all application names
export const useSearchedApplications = (applications: Application[], appNameParam: string) =>
    applications
        .filter((app) => appNameParam === '' || app.name.includes(appNameParam))
        .sort((a, b) => (a.team === b.team ? a.name?.localeCompare(b.name) : a.team?.localeCompare(b.team)));

// return all environment locks
export const useEnvironmentLocks = () =>
    useOverview(({ environments }) => {
        const locks = Object.values(environments).map((environment) =>
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
        );
        const locksFiltered = locks.filter((displayLock) => displayLock.length !== 0);
        return sortLocks(locksFiltered.flat(), 'descending');
    });

export const useFilteredEnvironmentLocks = (envName: string) =>
    useOverview(({ environments }) =>
        Object.values(
            Object.values(environments)
                .filter((environment) => environment.name === envName)
                .map((environment) => environment.locks)
                .reduce((acc, val) => ({ ...acc, ...val }), {})
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

// return all applications locks
export const useFilteredApplicationLocks = (appNameParam: string | null) =>
    useOverview(({ environments }) => {
        const finalLocks: DisplayLock[] = [];
        Object.values(environments)
            .map((environment) => ({ envName: environment.name, apps: environment.applications }))
            .forEach((app) => {
                Object.values(app.apps)
                    .map((myApp) => ({ environment: app.envName, appName: myApp.name, locks: myApp.locks }))
                    .forEach((lock) => {
                        Object.values(lock.locks).forEach((cena) =>
                            finalLocks.push({
                                date: cena.createdAt,
                                application: lock.appName,
                                environment: lock.environment,
                                lockId: cena.lockId,
                                message: cena.message,
                                authorName: cena.createdBy?.name,
                                authorEmail: cena.createdBy?.email,
                            } as DisplayLock)
                        );
                    });
            });
        const filteredLocks = finalLocks.filter((val) => searchCustomFilter(appNameParam, val.application));
        return sortLocks(filteredLocks, 'descending');
    });

export const searchCustomFilter = (queryContent: string | null, val: string | undefined) => {
    if (!!val && !!queryContent) {
        if (val.includes(queryContent)) {
            return val;
        }
        return null;
    } else {
        return val;
    }
};

// return all applications locks
export const useApplicationLocks = () =>
    useOverview(({ environments }) => {
        const finalLocks: DisplayLock[] = [];
        Object.values(environments)
            .map((environment) => ({ envName: environment.name, apps: environment.applications }))
            .forEach((app) => {
                Object.values(app.apps)
                    .map((myApp) => ({ environment: app.envName, appName: myApp.name, locks: myApp.locks }))
                    .forEach((lock) => {
                        Object.values(lock.locks).forEach((cena) =>
                            finalLocks.push({
                                date: cena.createdAt,
                                application: lock.appName,
                                environment: lock.environment,
                                lockId: cena.lockId,
                                message: cena.message,
                                authorName: cena.createdBy?.name,
                                authorEmail: cena.createdBy?.email,
                            } as DisplayLock)
                        );
                    });
            });
        return sortLocks(finalLocks, 'descending');
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

// Get release information for a version
export const useReleaseInfo = (app: string, version: number) =>
    useOverview(({ applications }) => {
        if (app !== '') {
            return applications[app].releases.filter((release) => release.version === version)[0];
        }
        return {};
    });
