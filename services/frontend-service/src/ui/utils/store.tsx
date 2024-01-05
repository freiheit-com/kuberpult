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
    GetFrontendConfigResponse,
    GetOverviewResponse,
    Priority,
    Release,
    StreamStatusResponse,
    Warning,
    GetGitTagsResponse,
    GetProductSummaryResponse,
} from '../../api/api';
import * as React from 'react';
import { useCallback, useMemo } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useIsAuthenticated } from '@azure/msal-react';
import { useApi } from './GrpcApi';

// see maxBatchActions in batch.go
export const maxBatchActions = 100;

export interface DisplayLock {
    date?: Date;
    environment: string;
    application?: string;
    message: string;
    lockId: string;
    authorName?: string;
    authorEmail?: string;
}

export const displayLockUniqueId = (displayLock: DisplayLock): string =>
    'dl-' + displayLock.lockId + '-' + displayLock.environment + '-' + displayLock.application;

type EnhancedOverview = GetOverviewResponse & { loaded: boolean };

const emptyOverview: EnhancedOverview = {
    applications: {},
    environmentGroups: [],
    gitRevision: '',
    loaded: false,
};
const [useOverview, UpdateOverview_] = createStore(emptyOverview);
export const UpdateOverview = UpdateOverview_; // we do not want to export "useOverview". The store.tsx should act like a facade to the data.

export const useOverviewLoaded = (): boolean => useOverview(({ loaded }) => loaded);
type TagsResponse = {
    response: GetGitTagsResponse;
    tagsReady: boolean;
};
type ProductSummaryResponse = {
    response: GetProductSummaryResponse;
    summaryReady: boolean;
};
const emptyBatch: BatchRequest = { actions: [] };
export const [useAction, UpdateAction] = createStore(emptyBatch);
const tagsResponse: GetGitTagsResponse = { tagData: [] };
export const refreshTags = (): void => {
    const api = useApi;
    api.tagsService()
        .GetGitTags({})
        .then((result: GetGitTagsResponse) => {
            updateTag.set({ response: result, tagsReady: true });
        })
        .catch((e) => {
            showSnackbarError(e.message);
        });
};
export const [useTag, updateTag] = createStore<TagsResponse>({ response: tagsResponse, tagsReady: false });

const summaryResponse: GetProductSummaryResponse = { productSummary: [] };
export const getSummary = (commitHash: string, environment: string, environmentGroup: string): void => {
    const api = useApi;
    api.tagsService()
        .GetProductSummary({ commitHash: commitHash, environment: environment, environmentGroup: environmentGroup })
        .then((result: GetProductSummaryResponse) => {
            updateSummary.set({ response: result, summaryReady: true });
        })
        .catch((e) => {
            showSnackbarError(e.message);
        });
};
export const [useSummary, updateSummary] = createStore<ProductSummaryResponse>({
    response: summaryResponse,
    summaryReady: false,
});

export const [_, PanicOverview] = createStore({ error: '' });

const randBase36 = (): string => Math.random().toString(36).substring(7);
export const randomLockId = (): string => 'ui-v2-' + randBase36();

export const useActions = (): BatchAction[] => useAction(({ actions }) => actions);
export const useTags = (): TagsResponse => useTag((res) => res);
export const useSummaryDisplay = (): ProductSummaryResponse => useSummary((res) => res);

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
    if (actions.length + 1 > maxBatchActions) {
        showSnackbarError('Maximum number of actions is ' + String(maxBatchActions));
        return;
    }
    // checking for duplicates
    switch (action.action?.$case) {
        case 'createEnvironmentLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'createEnvironmentLock' &&
                        action.action?.$case === 'createEnvironmentLock' &&
                        act.action.createEnvironmentLock.environment === action.action.createEnvironmentLock.environment
                    // lockId and message are ignored
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
                    // lockId and message are ignored
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
                            action.action.deleteEnvironmentApplicationLock.lockId &&
                        act.action.deleteEnvironmentApplicationLock.application ===
                            action.action.deleteEnvironmentApplicationLock.application
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
                    // version, lockBehavior and ignoreAllLocks are ignored
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

export const useOpenReleaseDialog = (app: string, version: number): (() => void) => {
    const [params, setParams] = useSearchParams();
    return useCallback(() => {
        params.set('dialog-app', app);
        params.set('dialog-version', version.toString());
        setParams(params);
    }, [app, params, setParams, version]);
};

export const useCloseReleaseDialog = (): (() => void) => {
    const [params, setParams] = useSearchParams();
    return useCallback(() => {
        params.delete('dialog-app');
        params.delete('dialog-version');
        setParams(params);
    }, [params, setParams]);
};

export const useReleaseDialogParams = (): { app: string | null; version: number | null } => {
    const [params] = useSearchParams();
    const app = params.get('dialog-app') ?? '';
    const version = +(params.get('dialog-version') ?? '');
    const valid = useOverview(({ applications }) =>
        applications[app] ? !!applications[app].releases.find((r) => r.version === version) : false
    );
    return valid ? { app, version } : { app: null, version: null };
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

export const useTeamFromApplication = (app: string): string | undefined =>
    useOverview(({ applications }) => applications[app]?.team?.trim());

// returns warnings from all apps
export const useAllWarnings = (): Warning[] =>
    useOverview(({ applications }) => Object.values(applications).flatMap((app) => app.warnings));

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

/**
 * returns the classname according to the priority of an environment, used to color environments
 */
export const getPriorityClassName = (environment: Environment): string =>
    'environment-priority-' + String(Priority[environment?.priority ?? Priority.UNRECOGNIZED]).toLowerCase();

// returns all application names
export const useSearchedApplications = (applications: Application[], appNameParam: string): Application[] =>
    applications
        .filter((app) => appNameParam === '' || app.name.includes(appNameParam))
        .sort((a, b) => (a.team === b.team ? a.name?.localeCompare(b.name) : a.team?.localeCompare(b.team)));

// return all applications locks
export const useFilteredApplicationLocks = (appNameParam: string | null): DisplayLock[] => {
    const finalLocks: DisplayLock[] = [];
    Object.values(useEnvironments())
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
                        })
                    );
                });
        });
    const filteredLocks = finalLocks.filter((val) => appNameParam === val.application);
    return sortLocks(filteredLocks, 'newestToOldest');
};

export const useLocksConflictingWithActions = (): AllLocks => {
    const allActions = useActions();
    const locks = useAllLocks();
    return {
        environmentLocks: locks.environmentLocks.filter((envLock: DisplayLock) => {
            const actions = allActions.filter((action) => {
                if (action.action?.$case === 'deploy') {
                    const env = action.action.deploy.environment;
                    if (envLock.environment === env) {
                        // found an env lock that matches
                        return true;
                    }
                }
                return false;
            });
            return actions.length > 0;
        }),
        appLocks: locks.appLocks.filter((envLock: DisplayLock) => {
            const actions = allActions.filter((action) => {
                if (action.action?.$case === 'deploy') {
                    const app = action.action.deploy.application;
                    const env = action.action.deploy.environment;
                    if (envLock.environment === env && envLock.application === app) {
                        // found an app lock that matches
                        return true;
                    }
                }
                return false;
            });
            return actions.length > 0;
        }),
    };
};

// return env lock IDs from given env
export const useFilteredEnvironmentLockIDs = (envName: string): string[] =>
    useEnvironments()
        .filter((env) => envName === '' || env.name === envName)
        .map((env) => Object.values(env.locks))
        .flat()
        .map((lock) => lock.lockId);

export const useEnvironmentLock = (lockId: string): DisplayLock => {
    const envs = useEnvironments();
    for (let i = 0; i < envs.length; i++) {
        const env = envs[i];
        for (const locksKey in env.locks) {
            const lock = env.locks[locksKey];
            if (lock.lockId === lockId) {
                return {
                    date: lock.createdAt,
                    message: lock.message,
                    lockId: lock.lockId,
                    authorName: lock.createdBy?.name,
                    authorEmail: lock.createdBy?.email,
                    environment: env.name,
                };
            }
        }
    }
    throw new Error('env lock with id not found: ' + lockId);
};

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

export type AllLocks = {
    environmentLocks: DisplayLock[];
    appLocks: DisplayLock[];
};

export const useAllLocks = (): AllLocks => {
    const envs = useEnvironments();
    const environmentLocks: DisplayLock[] = [];
    const appLocks: DisplayLock[] = [];
    envs.forEach((env: Environment) => {
        for (const locksKey in env.locks) {
            const lock = env.locks[locksKey];
            const displayLock: DisplayLock = {
                lockId: lock.lockId,
                date: lock.createdAt,
                environment: env.name,
                message: lock.message,
                authorName: lock.createdBy?.name,
                authorEmail: lock.createdBy?.email,
            };
            environmentLocks.push(displayLock);
        }
        for (const applicationsKey in env.applications) {
            const app = env.applications[applicationsKey];
            for (const locksKey in app.locks) {
                const lock = app.locks[locksKey];
                const displayLock: DisplayLock = {
                    lockId: lock.lockId,
                    application: app.name,
                    date: lock.createdAt,
                    environment: env.name,
                    message: lock.message,
                    authorName: lock.createdBy?.name,
                    authorEmail: lock.createdBy?.email,
                };
                appLocks.push(displayLock);
            }
        }
    });
    return {
        environmentLocks,
        appLocks,
    };
};

type DeleteActionData = {
    env: string;
    app: string | undefined;
    lockId: string;
};

const extractDeleteActionData = (batchAction: BatchAction): DeleteActionData | undefined => {
    if (batchAction.action?.$case === 'deleteEnvironmentApplicationLock') {
        return {
            env: batchAction.action.deleteEnvironmentApplicationLock.environment,
            app: batchAction.action.deleteEnvironmentApplicationLock.application,
            lockId: batchAction.action.deleteEnvironmentApplicationLock.lockId,
        };
    }
    if (batchAction.action?.$case === 'deleteEnvironmentLock') {
        return {
            env: batchAction.action.deleteEnvironmentLock.environment,
            app: undefined,
            lockId: batchAction.action.deleteEnvironmentLock.lockId,
        };
    }
    return undefined;
};

// returns all locks with the same ID
// that are not already in the cart
export const useLocksSimilarTo = (cartItemAction: BatchAction | undefined): AllLocks => {
    const allLocks = useAllLocks();
    const actions = useActions();

    if (!cartItemAction) {
        return { appLocks: [], environmentLocks: [] };
    }
    const data = extractDeleteActionData(cartItemAction);
    if (!data) {
        return {
            appLocks: [],
            environmentLocks: [],
        };
    }
    const isInCart = (lock: DisplayLock): boolean =>
        actions.find((cartAction: BatchAction): boolean => {
            const data = extractDeleteActionData(cartAction);
            if (!data) {
                return false;
            }
            return lock.lockId === data.lockId && lock.application === data.app && lock.environment === data.env;
        }) !== undefined;

    const resultLocks: AllLocks = {
        environmentLocks: [],
        appLocks: [],
    };
    allLocks.environmentLocks.forEach((envLock: DisplayLock) => {
        if (isInCart(envLock)) {
            return;
        }
        // if the id is the same, but we are on a different environment, or it's an app lock:
        if (envLock.lockId === data.lockId && (envLock.environment !== data.env || data.app !== undefined)) {
            resultLocks.environmentLocks.push(envLock);
        }
    });
    allLocks.appLocks.forEach((appLock: DisplayLock) => {
        if (isInCart(appLock)) {
            return;
        }
        // if the id is the same, but we are on a different environment or different app:
        if (appLock.lockId === data.lockId && (appLock.environment !== data.env || appLock.application !== data.app)) {
            resultLocks.appLocks.push(appLock);
        }
    });
    return resultLocks;
};

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
export const useRelease = (application: string, version: number): Release | undefined =>
    useOverview(({ applications }) => applications[application]?.releases?.find((r) => r.version === version));

export const useReleaseOrThrow = (application: string, version: number): Release => {
    const release = useRelease(application, version);
    if (!release) {
        throw new Error('Release cannot be found for app ' + application + ' version ' + version);
    }
    return release;
};

export const useReleaseOptional = (application: string, env: Environment): Release | undefined => {
    const x = env.applications[application];
    return useOverview(({ applications }) => {
        const version = x ? x.version : 0;
        const res = applications[application].releases.find((r) => r.version === version);
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

/**
 * returns the environments where an application is currently deployed
 */
export const useCurrentlyExistsAtGroup = (application: string): EnvironmentGroupExtended[] => {
    const environmentGroups: EnvironmentGroup[] = useEnvironmentGroups();
    return useMemo(() => {
        const envGroups: EnvironmentGroupExtended[] = [];
        environmentGroups.forEach((group: EnvironmentGroup) => {
            const envs = group.environments.filter((env) => env.applications[application]);
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
    }, [environmentGroups, application]);
};

// Get all releases for an app
export const useReleasesForApp = (app: string): Release[] =>
    useOverview(({ applications }) => applications[app]?.releases?.sort((a, b) => b.version - a.version));

// Get all release versions for an app
export const useVersionsForApp = (app: string): number[] => useReleasesForApp(app).map((rel) => rel.version);

// Navigate while keeping search params, returns new navigation url, and a callback function to navigate
export const useNavigateWithSearchParams = (to: string): { navURL: string; navCallback: () => void } => {
    const location = useLocation();
    const navigate = useNavigate();
    const queryParams = location?.search ?? '';
    const navURL = `${to}${queryParams}`;
    return {
        navURL: navURL,
        navCallback: React.useCallback(() => {
            navigate(navURL);
        }, [navURL, navigate]),
    };
};

type FrontendConfig = {
    configs: GetFrontendConfigResponse;
    configReady: boolean;
};

export const [useFrontendConfig, UpdateFrontendConfig] = createStore<FrontendConfig>({
    configs: {
        sourceRepoUrl: '',
        manifestRepoUrl: '',
        branch: '',
        kuberpultVersion: '0',
    },
    configReady: false,
});

export type GlobalLoadingState = {
    configReady: boolean;
    isAuthenticated: boolean;
    azureAuthEnabled: boolean;
    overviewLoaded: boolean;
};

// returns one loading state for all the calls done on startup, in order to render a spinner with details
export const useGlobalLoadingState = (): [boolean, GlobalLoadingState] => {
    const { configs, configReady } = useFrontendConfig((c) => c);
    const isAuthenticated = useIsAuthenticated();
    const azureAuthEnabled = configs.authConfig?.azureAuth?.enabled || false;
    const overviewLoaded = useOverviewLoaded();
    const everythingLoaded = overviewLoaded && configReady && (isAuthenticated || !azureAuthEnabled);
    return [
        everythingLoaded,
        {
            configReady,
            isAuthenticated,
            azureAuthEnabled,
            overviewLoaded,
        },
    ];
};

export const useKuberpultVersion = (): string => useFrontendConfig((configs) => configs.configs.kuberpultVersion);
export const useArgoCdBaseUrl = (): string | undefined =>
    useFrontendConfig((configs) => configs.configs.argoCd?.baseUrl);
export const useSourceRepoUrl = (): string | undefined => useFrontendConfig((configs) => configs.configs.sourceRepoUrl);
export const useManifestRepoUrl = (): string | undefined =>
    useFrontendConfig((configs) => configs.configs.manifestRepoUrl);
export const useBranch = (): string | undefined => useFrontendConfig((configs) => configs.configs.branch);

export type RolloutStatusApplication = {
    [environment: string]: StreamStatusResponse;
};

export type RolloutStatusStore = {
    enabled: boolean;
    applications: {
        [application: string]: RolloutStatusApplication;
    };
};

const [useEntireRolloutStatus, rolloutStatus] = createStore<RolloutStatusStore>({ enabled: false, applications: {} });

export const useRolloutStatus = (application: string): [boolean, RolloutStatusApplication] => {
    const enabled = useEntireRolloutStatus((data: RolloutStatusStore): boolean => data.enabled);
    const status = useEntireRolloutStatus(
        (data: RolloutStatusStore): RolloutStatusApplication => data.applications[application] ?? {}
    );
    return [enabled, status];
};

export const UpdateRolloutStatus = (ev: StreamStatusResponse): void => {
    rolloutStatus.set((data: RolloutStatusStore) => ({
        enabled: true,
        applications: {
            ...data.applications,
            [ev.application]: {
                ...(data.applications[ev.application] ?? {}),
                [ev.environment]: ev,
            },
        },
    }));
};

export const EnableRolloutStatus = (): void => {
    rolloutStatus.set({ enabled: true });
};

export const FlushRolloutStatus = (): void => {
    rolloutStatus.set({ enabled: false, applications: {} });
};
