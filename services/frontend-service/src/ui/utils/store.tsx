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
import { createStore } from 'react-use-sub';
import {
    Application,
    BatchAction,
    BatchRequest,
    Environment,
    EnvironmentGroup,
    GetOverviewResponse,
    Release,
} from '../../api/api';
import { useApi } from './GrpcApi';
import { useMemo } from 'react';
import { Empty } from '../../google/protobuf/empty';

export interface DisplayLock {
    date: Date;
    environment: string;
    application?: string;
    message: string;
    lockId: string;
    authorName: string;
    authorEmail: string;
}

const emptyOverview: GetOverviewResponse = { applications: {}, environments: {}, environmentGroups: [] };
export const [useOverview, UpdateOverview] = createStore(emptyOverview);

const emptyBatch: BatchRequest = { actions: [] };
export const [useAction, UpdateAction] = createStore(emptyBatch);

export const [_, PanicOverview] = createStore({ error: '' });

export const [useReleaseDialog, UpdateReleaseDialog] = createStore({ app: '', version: 0 });

export const useApplyActions = (): Promise<Empty> => useApi.batchService().ProcessBatch({ actions: useActions() });

export const useActions = (): BatchAction[] => useAction(({ actions }) => actions);

export const [useSidebar, UpdateSidebar] = createStore({ shown: false });

export enum SnackbarStatus {
    SUCCESS,
    WARN,
    ERROR,
}

export const [useSnackbar, UpdateSnackbar] = createStore({ show: false, status: SnackbarStatus.SUCCESS, content: '' });
export const showSnackbarSuccess = (content: string): void =>
    UpdateSnackbar.set({ show: true, status: SnackbarStatus.SUCCESS, content: content });
export const showSnackbarError = (content: string): void =>
    UpdateSnackbar.set({ show: true, status: SnackbarStatus.ERROR, content: content });
export const showSnackbarWarn = (content: string): void =>
    UpdateSnackbar.set({ show: true, status: SnackbarStatus.WARN, content: content });

export const useSidebarShown = (): boolean => useSidebar(({ shown }) => shown);

export const useNumberOfActions = (): number => useAction(({ actions }) => actions.length);

export const updateActions = (actions: BatchAction[]): void => {
    deleteAllActions();
    actions.forEach((action) => addAction(action));
};

export const appendAction = (actions: BatchAction[]): void => {
    actions.forEach((action) => addAction(action));
};

export const addAction = (action: BatchAction): void => {
    const actions = UpdateAction.get().actions;
    // checking for duplicates
    switch (action.action?.$case) {
        case 'createEnvironmentLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'createEnvironmentLock' &&
                        action.action?.$case === 'createEnvironmentLock' &&
                        act.action.createEnvironmentLock.environment === action.action.createEnvironmentLock.environment
                )
            )
                return;
            break;
        case 'deleteEnvironmentLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'deleteEnvironmentLock' &&
                        action.action?.$case === 'deleteEnvironmentLock' &&
                        act.action.deleteEnvironmentLock.environment ===
                            action.action.deleteEnvironmentLock.environment &&
                        act.action.deleteEnvironmentLock.lockId === action.action.deleteEnvironmentLock.lockId
                )
            )
                return;
            break;
        case 'createEnvironmentApplicationLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'createEnvironmentApplicationLock' &&
                        action.action?.$case === 'createEnvironmentApplicationLock' &&
                        act.action.createEnvironmentApplicationLock.application ===
                            action.action.createEnvironmentApplicationLock.application &&
                        act.action.createEnvironmentApplicationLock.environment ===
                            action.action.createEnvironmentApplicationLock.environment
                )
            )
                return;
            break;
        case 'deleteEnvironmentApplicationLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'deleteEnvironmentApplicationLock' &&
                        action.action?.$case === 'deleteEnvironmentApplicationLock' &&
                        act.action.deleteEnvironmentApplicationLock.environment ===
                            action.action.deleteEnvironmentApplicationLock.environment &&
                        act.action.deleteEnvironmentApplicationLock.lockId ===
                            action.action.deleteEnvironmentApplicationLock.lockId
                )
            )
                return;
            break;
        case 'deploy':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'deploy' &&
                        action.action?.$case === 'deploy' &&
                        act.action.deploy.application === action.action.deploy.application &&
                        act.action.deploy.environment === action.action.deploy.environment
                )
            )
                return;
            break;
        case 'undeploy':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'undeploy' &&
                        action.action?.$case === 'undeploy' &&
                        act.action.undeploy.application === action.action.undeploy.application
                )
            )
                return;
            break;
        case 'prepareUndeploy':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'prepareUndeploy' &&
                        action.action?.$case === 'prepareUndeploy' &&
                        act.action.prepareUndeploy.application === action.action.prepareUndeploy.application
                )
            )
                return;
            break;
    }
    UpdateAction.set({ actions: [...UpdateAction.get().actions, action] });
    UpdateSidebar.set({ shown: true });
};

export const updateReleaseDialog = (app: string, version: number): void => {
    UpdateReleaseDialog.set({ app: app, version: version });
};
export const deleteAllActions = (): void => {
    UpdateAction.set({ actions: [] });
};

export const deleteAction = (action: BatchAction): void => {
    UpdateAction.set(({ actions }) => ({
        // create comparison function
        actions: actions.filter((act) => JSON.stringify(act).localeCompare(JSON.stringify(action))),
    }));
};
// returns all application names
// doesn't return empty team names (i.e.: '')
// doesn't return repeated team names
export const useTeamNames = (): string[] =>
    useOverview(({ applications }) => [
        ...new Set(
            Object.values(applications)
                .map((app: Application) => app.team.trim() || '<No Team>')
                .sort((a, b) => a.localeCompare(b))
        ),
    ]);

export const useTeamFromApplication = (app: string): string =>
    useOverview(({ applications }) => applications[app]?.team?.trim() || '<No Team>');

// returns applications filtered by dropdown and sorted by team name and then by app name
export const useFilteredApps = (teams: string[]): Application[] =>
    useOverview(({ applications }) =>
        Object.values(applications).filter(
            (app) => teams.length === 0 || teams.includes(app.team.trim() || '<No Team>')
        )
    );

export const useEnvironmentGroups = (): EnvironmentGroup[] => useOverview(({ environmentGroups }) => environmentGroups);

/**
 * returns all environments
 */
export const useEnvironments = (): Environment[] =>
    useOverview(({ environmentGroups }) => environmentGroups.flatMap((envGroup) => envGroup.environments));

/**
 * returns all environment names
 */
export const useEnvironmentNames = (): string[] => useEnvironments().map((env) => env.name);

// returns all application names
export const useSearchedApplications = (applications: Application[], appNameParam: string): Application[] =>
    applications
        .filter((app) => appNameParam === '' || app.name.includes(appNameParam))
        .sort((a, b) => (a.team === b.team ? a.name?.localeCompare(b.name) : a.team?.localeCompare(b.team)));

// return all applications locks
export const useFilteredApplicationLocks = (appNameParam: string | null): DisplayLock[] =>
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
        return sortLocks(filteredLocks, 'newestToOldest');
    });

// return all environment locks
export const useEnvironmentLocks = (): DisplayLock[] =>
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
        return sortLocks(locksFiltered.flat(), 'oldestToNewest');
    });

// return all env lock IDs
export const useEnvironmentLockIDs = (): string[] =>
    useOverview(({ environments }) =>
        Object.values(environments)
            .map((env) => Object.values(env.locks))
            .flat()
            .map((lock) => lock.lockId)
    );

// return env lock IDs from given env
export const useFilteredEnvironmentLockIDs = (envName: string): string[] =>
    useOverview(({ environments }) =>
        Object.values(environments)
            .filter((env) => envName === '' || env.name === envName)
            .map((env) => Object.values(env.locks))
            .flat()
            .map((lock) => lock.lockId)
    );

export const useFilteredEnvironmentLocks = (envName: string): string[] =>
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

export const useEnvironmentLock = (lockId: string): DisplayLock =>
    ({
        ...useOverview(
            ({ environments }) =>
                Object.values(
                    Object.values(environments)
                        .map((env) => env.locks)
                        .reduce((acc, val) => ({ ...acc, ...val }))
                )
                    .map((lock) => ({
                        date: lock.createdAt,
                        message: lock.message,
                        lockId: lock.lockId,
                        authorName: lock.createdBy?.name,
                        authorEmail: lock.createdBy?.email,
                    }))
                    .find((lock) => lock.lockId === lockId)!
        ),
        environment: useOverview(({ environments }) =>
            Object.values(environments).find((env) => Object.values(env.locks).find((lock) => lock.lockId === lockId))
        )?.name,
    } as DisplayLock);

export const searchCustomFilter = (queryContent: string | null, val: string | undefined): string => {
    if (!!val && !!queryContent) {
        if (val.includes(queryContent)) {
            return val;
        }
        return '';
    } else {
        return val || '';
    }
};

// return app lock IDs
export const useApplicationLockIDs = (): string[] =>
    useOverview(({ environments }) =>
        Object.values(environments)
            .map((env) => Object.values(env.applications))
            .flat()
            .map((app) => Object.values(app.locks))
            .flat()
            .map((lock) => lock.lockId)
    );

export const useApplicationLock = (lockId: string): DisplayLock =>
    ({
        ...useOverview(
            ({ environments }) =>
                Object.values(
                    Object.values(environments)
                        .map((env) => Object.values(env.applications))
                        .flat()
                        .map((app) => app.locks)
                        .reduce((acc, val) => ({ ...acc, ...val }))
                )
                    .map((lock) => ({
                        date: lock.createdAt,
                        message: lock.message,
                        lockId: lock.lockId,
                        authorName: lock.createdBy?.name,
                        authorEmail: lock.createdBy?.email,
                    }))
                    .find((lock) => lock.lockId === lockId)!
        ),
        environment: useOverview(({ environments }) =>
            Object.values(environments).find((env) =>
                Object.values(env.applications).find((app) =>
                    Object.values(app.locks).find((lock) => lock.lockId === lockId)
                )
            )
        )?.name,
        application: useOverview(({ environments }) =>
            Object.values(environments)
                .map((env) => Object.values(env.applications))
                .flat()
                .find((app) => Object.values(app.locks).find((lock) => lock.lockId === lockId))
        )?.name,
    } as DisplayLock);

export const sortLocks = (displayLocks: DisplayLock[], sorting: 'oldestToNewest' | 'newestToOldest'): DisplayLock[] => {
    const sortMethod = sorting === 'newestToOldest' ? -1 : 1;
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
export const useRelease = (application: string, version: number): Release =>
    useOverview(({ applications }) => applications[application].releases.find((r) => r.version === version)!);

export const useReleaseOptional = (application: string, env: Environment): Release | undefined => {
    const x = env.applications[application];
    return useOverview(({ applications }) => {
        const version = x ? x.version : 0;
        const res = applications[application].releases.find((r) => r.version === version)!;
        if (!x) {
            return undefined;
        }
        return res;
    });
};

// returns the release versions that are currently deployed to at least one environment
export const useDeployedReleases = (application: string): number[] =>
    [
        ...new Set(
            Object.values(useEnvironments())
                .filter((env) => env.applications[application])
                .map((env) => env.applications[application].version)
        ),
    ]
        .sort((a, b) => b - a)
        .filter((version) => version !== 0); // 0 means "not deployed", so we filter those out

export type EnvironmentGroupExtended = EnvironmentGroup & { numberOfEnvsInGroup: number };

/**
 * returns the environments where a release is currently deployed
 */
export const useCurrentlyDeployedAtGroup = (application: string, version: number): EnvironmentGroupExtended[] => {
    const environmentGroups: EnvironmentGroup[] = useEnvironmentGroups();
    return useMemo(() => {
        const envGroups: EnvironmentGroupExtended[] = [];
        environmentGroups.forEach((group: EnvironmentGroup) => {
            const envs = group.environments.filter(
                (env) => env.applications[application] && env.applications[application].version === version
            );
            if (envs.length > 0) {
                // we need to make a copy of the group here, because we want to remove some envs.
                // but that should not have any effect on the group saved in the store.
                const groupCopy: EnvironmentGroupExtended = {
                    environmentGroupName: group.environmentGroupName,
                    environments: envs,
                    distanceToUpstream: group.distanceToUpstream,
                    numberOfEnvsInGroup: group.environments.length,
                };
                envGroups.push(groupCopy);
            }
        });
        return envGroups;
    }, [environmentGroups, application, version]);
};

// Get release information for a version
export const useReleaseInfo = (app: string, version: number): Release =>
    useOverview(({ applications }) => {
        const releaseInfo = applications[app]?.releases.filter((release) => release.version === version)[0];
        if (!releaseInfo) {
            return {} as Release;
        }
        return releaseInfo;
    });

// Get all releases for an app
export const useReleasesForApp = (app: string): Release[] =>
    useOverview(({ applications }) => applications[app]?.releases.sort((a, b) => b.version - a.version));

// Get all release versions for an app
export const useVersionsForApp = (app: string): number[] => useReleasesForApp(app).map((rel) => rel.version);
