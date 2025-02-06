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
import { createStore } from 'react-use-sub';
import {
    AllAppLocks,
    AllTeamLocks,
    BatchAction,
    BatchRequest,
    EnvApp,
    Environment,
    EnvironmentGroup,
    GetAppDetailsResponse,
    GetCommitInfoResponse,
    GetEnvironmentConfigResponse,
    GetFailedEslsResponse,
    GetFrontendConfigResponse,
    GetGitSyncStatusResponse,
    GetGitTagsResponse,
    GetOverviewResponse,
    GetReleaseTrainPrognosisResponse,
    Locks,
    OverviewApplication,
    Priority,
    Release,
    RolloutStatus,
    StreamStatusResponse,
    Warning,
} from '../../api/api';
import * as React from 'react';
import { useCallback, useMemo } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useIsAuthenticated } from '@azure/msal-react';
import { useApi } from './GrpcApi';
import { AuthHeader } from './AzureAuthProvider';
import { isTokenValid, LoginPage } from '../utils/DexAuthProvider';
import { LoadingStateSpinner } from '../utils/LoadingStateSpinner';
import { GitSyncStatus } from '../components/GitSyncStatusDescription/GitSyncStatusDescription';

// see maxBatchActions in batch.go
export const maxBatchActions = 100;

export interface DisplayLock {
    date?: Date;
    environment: string;
    application?: string;
    team?: string;
    message: string;
    lockId: string;
    authorName?: string;
    authorEmail?: string;
}

export const displayLockUniqueId = (displayLock: DisplayLock): string =>
    'dl-' +
    displayLock.lockId +
    '-' +
    displayLock.environment +
    '-' +
    (displayLock.application ? displayLock.application : displayLock.team);

type EnhancedOverview = GetOverviewResponse & { [key: string]: unknown; loaded: boolean };

const emptyOverview: EnhancedOverview = {
    lightweightApps: [],
    environmentGroups: [],
    gitRevision: '',
    loaded: false,
    branch: '',
    manifestRepoUrl: '',
};

const [useOverview, UpdateOverview_] = createStore(emptyOverview);
export const UpdateOverview = UpdateOverview_; // we do not want to export "useOverview". The store.tsx should act like a facade to the data.

export const useOverviewLoaded = (): boolean => useOverview(({ loaded }) => loaded);

export const emptyAppLocks: { [key: string]: AllAppLocks } = {};
export const [useAllApplicationLocks, UpdateAllApplicationLocks] = createStore<{ [key: string]: AllAppLocks }>(
    emptyAppLocks
);
export const emptyEnvLocks: { [key: string]: Locks } = {};
export const emptyTeamLocks: { [key: string]: AllTeamLocks } = {};
export const [useAllEnvLocks, updateAllEnvLocks] = createStore<{
    allEnvLocks: { [key: string]: Locks };
    allTeamLocks: { [key: string]: AllTeamLocks };
}>({
    allEnvLocks: emptyEnvLocks,
    allTeamLocks: emptyTeamLocks,
});

type TagsResponse = {
    response: GetGitTagsResponse;
    tagsReady: boolean;
};

export enum CommitInfoState {
    LOADING,
    READY,
    ERROR,
    NOTFOUND,
}

export enum AppDetailsState {
    LOADING,
    READY,
    ERROR,
    NOTFOUND,
    NOTREQUESTED = 4,
}

export type CommitInfoResponse = {
    response: GetCommitInfoResponse | undefined;
    commitInfoReady: CommitInfoState;
};

export type AppDetailsResponse = {
    details: GetAppDetailsResponse | undefined;
    appDetailState: AppDetailsState;
    updatedAt: Date | undefined;
    errorMessage: string | undefined;
};

export enum FailedEslsState {
    LOADING,
    READY,
    ERROR,
    NOTFOUND,
}

export type FailedEslsResponse = {
    response: GetFailedEslsResponse | undefined;
    failedEslsReady: FailedEslsState;
};

export enum ReleaseTrainPrognosisState {
    LOADING,
    READY,
    ERROR,
    NOTFOUND,
}
export type ReleaseTrainPrognosisResponse = {
    response: GetReleaseTrainPrognosisResponse | undefined;
    releaseTrainPrognosisReady: ReleaseTrainPrognosisState;
};
const emptyBatch: BatchRequest & { [key: string]: unknown } = { actions: [] };

export const [useAction, UpdateAction] = createStore(emptyBatch);
const tagsResponse: GetGitTagsResponse = { tagData: [] };
export const refreshTags = (): void => {
    const api = useApi;
    api.gitService()
        .GetGitTags({})
        .then((result: GetGitTagsResponse) => {
            updateTag.set({ response: result, tagsReady: true });
        })
        .catch((e) => {
            showSnackbarError(e.message);
        });
};
export const [useTag, updateTag] = createStore<TagsResponse>({ response: tagsResponse, tagsReady: false });

export const emptyDetails: { [key: string]: AppDetailsResponse } = {};
export const [useAppDetails, updateAppDetails] = createStore<{ [key: string]: AppDetailsResponse }>(emptyDetails);

const emptyWarnings: { [key: string]: Warning[] } = {};
export const [useWarnings, updateWarnings] = createStore<{ [key: string]: Warning[] }>(emptyWarnings);

export const useAllWarningsAllApps = (): Warning => useWarnings((map) => map);

export const getAppDetails = (appName: string, authHeader: AuthHeader): void => {
    const details = updateAppDetails.get();
    details[appName] = {
        details: details[appName] ? details[appName].details : undefined,
        appDetailState: AppDetailsState.LOADING,
        updatedAt: undefined,
        errorMessage: '',
    };
    updateAppDetails.set(details);
    useApi
        .overviewService()
        .GetAppDetails({ appName: appName }, authHeader)
        .then((result: GetAppDetailsResponse) => {
            const d = updateAppDetails.get();
            d[appName] = {
                details: result,
                appDetailState: AppDetailsState.READY,
                updatedAt: new Date(Date.now()),
                errorMessage: '',
            };
            updateAppDetails.set(d);
        })
        .catch((e) => {
            const GrpcErrorNotFound = 5;
            if (e.code === GrpcErrorNotFound) {
                details[appName] = {
                    details: undefined,
                    appDetailState: AppDetailsState.NOTFOUND,
                    updatedAt: new Date(Date.now()),
                    errorMessage: e.message,
                };
            } else {
                details[appName] = {
                    details: undefined,
                    appDetailState: AppDetailsState.ERROR,
                    updatedAt: new Date(Date.now()),
                    errorMessage: e.message,
                };
            }
            updateAppDetails.set(details);
            showSnackbarError(e.message);
        });
};

export const getCommitInfo = (commitHash: string, pageNumber: number, authHeader: AuthHeader): void => {
    useApi
        .gitService()
        .GetCommitInfo({ commitHash: commitHash, pageNumber: pageNumber }, authHeader)
        .then((result: GetCommitInfoResponse) => {
            const requestResult: GetCommitInfoResponse = structuredClone(result);
            const oldEvents = updateCommitInfo.get().response?.events.slice() ?? [];
            requestResult.events = oldEvents.concat(requestResult.events).slice();
            updateCommitInfo.set({ response: requestResult, commitInfoReady: CommitInfoState.READY });
        })
        .catch((e) => {
            const GrpcErrorNotFound = 5;
            if (e.code === GrpcErrorNotFound) {
                updateCommitInfo.set({ response: undefined, commitInfoReady: CommitInfoState.NOTFOUND });
            } else {
                showSnackbarError(e.message);
                updateCommitInfo.set({ response: undefined, commitInfoReady: CommitInfoState.ERROR });
            }
        });
};
export const [useCommitInfo, updateCommitInfo] = createStore<CommitInfoResponse>({
    response: undefined,
    commitInfoReady: CommitInfoState.LOADING,
});

export const getFailedEsls = (authHeader: AuthHeader): void => {
    useApi
        .eslService()
        .GetFailedEsls({}, authHeader)
        .then((result: GetFailedEslsResponse) => {
            updateFailedEsls.set({ response: result, failedEslsReady: FailedEslsState.READY });
        })
        .catch((e) => {
            const GrpcErrorNotFound = 3;
            if (e.code === GrpcErrorNotFound) {
                updateFailedEsls.set({ response: undefined, failedEslsReady: FailedEslsState.NOTFOUND });
            } else {
                showSnackbarError(e.message);
                updateFailedEsls.set({ response: undefined, failedEslsReady: FailedEslsState.ERROR });
            }
        });
};
export const [useFailedEsls, updateFailedEsls] = createStore<FailedEslsResponse>({
    response: undefined,
    failedEslsReady: FailedEslsState.LOADING,
});

export const getReleaseTrainPrognosis = (envName: string, authHeader: AuthHeader): void => {
    useApi
        .releaseTrainPrognosisService()
        .GetReleaseTrainPrognosis({ target: envName }, authHeader)
        .then((result: GetReleaseTrainPrognosisResponse) => {
            updateReleaseTrainPrognosis.set({
                response: result,
                releaseTrainPrognosisReady: ReleaseTrainPrognosisState.READY,
            });
        })
        .catch((e) => {
            const GrpcErrorNotFound = 3;
            if (e.code === GrpcErrorNotFound) {
                updateReleaseTrainPrognosis.set({
                    response: undefined,
                    releaseTrainPrognosisReady: ReleaseTrainPrognosisState.NOTFOUND,
                });
            } else {
                showSnackbarError(e.message);
                updateReleaseTrainPrognosis.set({
                    response: undefined,
                    releaseTrainPrognosisReady: ReleaseTrainPrognosisState.ERROR,
                });
            }
        });
};

export const [useReleaseTrainPrognosis, updateReleaseTrainPrognosis] = createStore<ReleaseTrainPrognosisResponse>({
    response: undefined,
    releaseTrainPrognosisReady: ReleaseTrainPrognosisState.LOADING,
});

export const [_, PanicOverview] = createStore({ error: '' });

const randBase36 = (): string => Math.random().toString(36).substring(7);
export const randomLockId = (): string => 'ui-v2-' + randBase36();

export const useActions = (): BatchAction[] => useAction(({ actions }) => actions);
export const useTags = (): TagsResponse => useTag((res) => res);

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
    let isDuplicate = false;
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
                isDuplicate = true;
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
                isDuplicate = true;
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
                isDuplicate = true;
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
                isDuplicate = true;
            break;
        case 'createEnvironmentTeamLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'createEnvironmentTeamLock' &&
                        action.action?.$case === 'createEnvironmentTeamLock' &&
                        act.action.createEnvironmentTeamLock.environment ===
                            action.action.createEnvironmentTeamLock.environment &&
                        act.action.createEnvironmentTeamLock.lockId ===
                            action.action.createEnvironmentTeamLock.lockId &&
                        act.action.createEnvironmentTeamLock.team === action.action.createEnvironmentTeamLock.team
                    // lockId and message are ignored
                )
            )
                isDuplicate = true;
            break;
        case 'deleteEnvironmentTeamLock':
            if (
                actions.some(
                    (act) =>
                        act.action?.$case === 'deleteEnvironmentTeamLock' &&
                        action.action?.$case === 'deleteEnvironmentTeamLock' &&
                        act.action.deleteEnvironmentTeamLock.environment ===
                            action.action.deleteEnvironmentTeamLock.environment &&
                        act.action.deleteEnvironmentTeamLock.lockId ===
                            action.action.deleteEnvironmentTeamLock.lockId &&
                        act.action.deleteEnvironmentTeamLock.team === action.action.deleteEnvironmentTeamLock.team
                )
            )
                isDuplicate = true;
            break;
        case 'deploy':
            if (
                actions.some(
                    (act) =>
                        (act.action?.$case === 'deploy' &&
                            action.action?.$case === 'deploy' &&
                            act.action.deploy.application === action.action.deploy.application &&
                            act.action.deploy.environment === action.action.deploy.environment) ||
                        act.action?.$case === 'releaseTrain'
                    // version, lockBehavior and ignoreAllLocks are ignored
                )
            )
                isDuplicate = true;

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
                isDuplicate = true;
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
                isDuplicate = true;
            break;
        case 'releaseTrain':
            // only allow one release train at a time to avoid conflicts or if there are existing deploy actions
            if (actions.some((act) => act.action?.$case === 'releaseTrain' || act.action?.$case === 'deploy')) {
                showSnackbarError(
                    'Can only have one release train action at a time and can not have deploy actions in parrallel'
                );
                return;
            }

            break;
    }

    const shouldCancel = ['deploy', 'createEnvironmentApplicationLock', 'deleteEnvironmentApplicationLock'];
    if (isDuplicate && shouldCancel.includes(action.action?.$case || '')) {
        deleteAction(action);
    } else if (isDuplicate) {
        showSnackbarSuccess('This action was already added.');
    } else {
        UpdateAction.set({ actions: [...UpdateAction.get().actions, action] });
    }
};

export const useOpenReleaseDialog = (app: string, version: number): (() => void) => {
    const [params, setParams] = useSearchParams();
    return useCallback(() => {
        params.set('dialog-app', app);
        params.set('dialog-version', version.toString());
        setParams(params);
    }, [app, params, setParams, version]);
};

export const useAppDetailsForApp = (app: string): AppDetailsResponse => useAppDetails((map) => map[app]);

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

    const response = useAppDetailsForApp(app);

    if (!response || !response.details) {
        return { app: null, version: null };
    }
    const appDetails = response.details;

    const valid = !!appDetails.application?.releases.find((r) => r.version === version);
    return valid ? { app, version } : { app: null, version: null };
};

export const deleteAllActions = (): void => {
    UpdateAction.set({ actions: [] });
};

export const deleteAction = (action: BatchAction): void => {
    UpdateAction.set(({ actions }) => ({
        // create comparison function
        actions: actions.filter((act) => JSON.stringify(act).localeCompare(JSON.stringify(action))),
        //actions: [],
    }));
};
// returns all application names
// doesn't return empty team names (i.e.: '')
// doesn't return repeated team names
export const useTeamNames = (): string[] =>
    useOverview(({ lightweightApps }) => [
        ...new Set(
            Object.values(lightweightApps)
                .map((app: OverviewApplication) => app.team.trim() || '<No Team>')
                .sort((a, b) => a.localeCompare(b))
        ),
    ]);
export const useApplications = (): OverviewApplication[] => useOverview(({ lightweightApps }) => lightweightApps);

export const useTeamFromApplication = (app: string): string | undefined =>
    useOverview(({ lightweightApps }) => lightweightApps.find((data) => data.name === app)?.team);

// returns warnings from all apps
export const useAllWarnings = (): Warning[] => {
    const names = useOverview(({ lightweightApps }) => lightweightApps).map((curr) => curr.name);
    const allAppDetails = updateAppDetails.get();
    return names
        .map((name) => {
            const resp = allAppDetails[name];
            if (resp === undefined || !allAppDetails[name].details) {
                return [];
            } else {
                const app = resp.details?.application;
                if (app === undefined) {
                    return [];
                } else {
                    return app.warnings;
                }
            }
        })
        .flatMap((curr) => curr);
};

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

export const useTeamLocks = (allApps: OverviewApplication[]): DisplayLock[] => {
    const allTeamLocks = useAllEnvLocks((map) => map.allTeamLocks);
    return Object.keys(allTeamLocks)
        .map((env) =>
            allApps
                .map((app) =>
                    allTeamLocks[env].teamLocks[app.team]
                        ? allTeamLocks[env].teamLocks[app.team].locks.map((lock) => ({
                              date: lock.createdAt,
                              environment: env,
                              team: app.team,
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
        .filter(
            (value: DisplayLock, index: number, self: DisplayLock[]) =>
                index ===
                self.findIndex(
                    (t: DisplayLock) =>
                        t.lockId === value.lockId && t.team === value.team && t.environment === value.environment
                )
        );
};

export const useAppLocks = (allAppLocks: Map<string, AllAppLocks>): DisplayLock[] => {
    const allAppLocksDisplay: DisplayLock[] = [];
    allAppLocks.forEach((appLocksForEnv, env): void => {
        const currAppLocks = new Map<string, Locks>(Object.entries(appLocksForEnv.appLocks));
        currAppLocks.forEach((currentAppInfo, app) => {
            currentAppInfo.locks.map((lock) =>
                allAppLocksDisplay.push({
                    date: lock.createdAt,
                    environment: env,
                    application: app,
                    lockId: lock.lockId,
                    message: lock.message,
                    authorName: lock.createdBy?.name,
                    authorEmail: lock.createdBy?.email,
                })
            );
        });
    });
    return allAppLocksDisplay;
};
/**
 * returns the classname according to the priority of an environment, used to color environments
 */
export const getPriorityClassName = (envOrGroup: Environment | EnvironmentGroup): string =>
    'environment-priority-' + String(Priority[envOrGroup?.priority ?? Priority.UNRECOGNIZED]).toLowerCase();

// filter for apps included in the selected teams
const applicationsMatchingTeam = (applications: OverviewApplication[], teams: string[]): OverviewApplication[] =>
    applications.filter((app) => teams.length === 0 || teams.includes(app.team.trim() || '<No Team>'));

//filter for all application names that have warnings
export const applicationsWithWarnings = (applications: OverviewApplication[]): OverviewApplication[] =>
    applications
        .map((app) => {
            const d = updateAppDetails.get()[app.name];
            if (d === undefined || !updateAppDetails.get()[app.name].details) {
                return [];
            } else {
                const currApp = d.details?.application;
                if (currApp === undefined) {
                    return [];
                } else {
                    return currApp.warnings.length > 0 ? [app] : [];
                }
            }
        })
        .flatMap((curr) => curr);

// filters given apps with the search terms or all for the empty string
const applicationsMatchingName = (applications: OverviewApplication[], appNameParam: string): OverviewApplication[] =>
    applications.filter((app) => appNameParam === '' || app.name.includes(appNameParam));

// sorts given apps by team
const applicationsSortedByTeam = (applications: OverviewApplication[]): OverviewApplication[] =>
    applications.sort((a, b) => (a.team === b.team ? a.name?.localeCompare(b.name) : a.team?.localeCompare(b.team)));

// returns applications to show on the home page
export const useApplicationsFilteredAndSorted = (
    teams: string[],
    withWarningsOnly: boolean,
    nameIncludes: string
): OverviewApplication[] => {
    const all = useOverview(({ lightweightApps }) => Object.values(lightweightApps));
    const allMatchingTeam = applicationsMatchingTeam(all, teams);
    const allMatchingTeamAndWarnings = withWarningsOnly ? applicationsWithWarnings(allMatchingTeam) : allMatchingTeam;
    const allMatchingTeamAndWarningsAndName = applicationsMatchingName(allMatchingTeamAndWarnings, nameIncludes);
    return applicationsSortedByTeam(allMatchingTeamAndWarningsAndName);
};

export interface DisplayApplicationLock {
    lock: DisplayLock;
    application: string;
    environment: Environment;
    environmentGroup: EnvironmentGroup;
}

export const useDisplayApplicationLocks = (appName: string): DisplayApplicationLock[] => {
    const envGroups = useEnvironmentGroups();
    const allAppLocks = useAllApplicationLocks((map) => map);
    const appLocks = useAppLocks(new Map(Object.entries(allAppLocks)));
    return useMemo(() => {
        const finalLocks: DisplayApplicationLock[] = [];
        Object.values(envGroups).forEach((envGroup) => {
            Object.values(envGroup.environments).forEach((env: Environment) =>
                appLocks.forEach((currentLock) =>
                    currentLock.application &&
                    currentLock.application === appName &&
                    currentLock.environment === env.name
                        ? finalLocks.push({
                              lock: {
                                  date: currentLock.date,
                                  application: appName,
                                  environment: env.name,
                                  lockId: currentLock.lockId,
                                  message: currentLock.message,
                                  authorName: currentLock.authorName,
                                  authorEmail: currentLock.authorEmail,
                              },
                              application: appName,
                              environment: env,
                              environmentGroup: envGroup,
                          })
                        : []
                )
            );
        });
        finalLocks.sort((a: DisplayApplicationLock, b: DisplayApplicationLock) => {
            if ((a.lock.date ?? new Date(0)) < (b.lock.date ?? new Date(0))) return 1;
            else if ((a.lock.date ?? new Date(0)) > (b.lock.date ?? new Date(0))) return -1;
            return 0;
        });
        return finalLocks;
    }, [appName, envGroups, appLocks]);
};

export const useLocksConflictingWithActions = (): AllLocks => {
    const allActions = useActions();
    const locks = useAllLocks();
    const appMap = useApplications();

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
        teamLocks: locks.teamLocks.filter((teamLock: DisplayLock) => {
            const actions = allActions.filter((action) => {
                if (action.action?.$case === 'deploy') {
                    const app = action.action.deploy.application;
                    const env = action.action.deploy.environment;
                    const appTeam = appMap.find((curr) => curr.name === app)?.team;
                    if (teamLock.environment === env && teamLock.team === appTeam) {
                        // found a team lock that matches
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
    (useAllEnvLocks((map) => map.allEnvLocks)[envName]?.locks ?? []).map((lock) => lock.lockId);

export const useEnvironmentLock = (lockId: string): DisplayLock => {
    const envLocks = useAllEnvLocks((map) => map.allEnvLocks);
    for (const env in envLocks) {
        for (const lock of envLocks[env]?.locks ?? []) {
            if (lock.lockId === lockId) {
                return {
                    date: lock.createdAt,
                    message: lock.message,
                    lockId: lock.lockId,
                    authorName: lock.createdBy?.name,
                    authorEmail: lock.createdBy?.email,
                    environment: env,
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
    teamLocks: DisplayLock[];
};

export const useAllLocks = (): AllLocks => {
    const allApps = useApplications();
    const allAppLocks = useAllApplicationLocks((map) => map);
    const allEnvLocks = useAllEnvLocks((map) => map.allEnvLocks);
    const teamLocks = useTeamLocks(allApps);
    const environmentLocks: DisplayLock[] = [];
    const appLocks = useAppLocks(new Map(Object.entries(allAppLocks)));
    Object.keys(allEnvLocks).forEach((env: string) => {
        for (const lock of allEnvLocks[env]?.locks ?? []) {
            const displayLock: DisplayLock = {
                lockId: lock.lockId,
                date: lock.createdAt,
                environment: env,
                message: lock.message,
                authorName: lock.createdBy?.name,
                authorEmail: lock.createdBy?.email,
            };
            environmentLocks.push(displayLock);
        }
    });
    return {
        environmentLocks,
        appLocks,
        teamLocks,
    };
};

type DeleteActionData = {
    env: string;
    app: string | undefined;
    team: string | undefined;
    lockId: string;
};

const extractDeleteActionData = (batchAction: BatchAction): DeleteActionData | undefined => {
    if (batchAction.action?.$case === 'deleteEnvironmentApplicationLock') {
        return {
            env: batchAction.action.deleteEnvironmentApplicationLock.environment,
            app: batchAction.action.deleteEnvironmentApplicationLock.application,
            team: undefined,
            lockId: batchAction.action.deleteEnvironmentApplicationLock.lockId,
        };
    }
    if (batchAction.action?.$case === 'deleteEnvironmentLock') {
        return {
            env: batchAction.action.deleteEnvironmentLock.environment,
            app: undefined,
            team: undefined,
            lockId: batchAction.action.deleteEnvironmentLock.lockId,
        };
    }
    if (batchAction.action?.$case === 'deleteEnvironmentTeamLock') {
        return {
            env: batchAction.action.deleteEnvironmentTeamLock.environment,
            app: undefined,
            team: batchAction.action.deleteEnvironmentTeamLock.team,
            lockId: batchAction.action.deleteEnvironmentTeamLock.lockId,
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
        return { appLocks: [], environmentLocks: [], teamLocks: [] };
    }
    const data = extractDeleteActionData(cartItemAction);
    if (!data) {
        return {
            appLocks: [],
            environmentLocks: [],
            teamLocks: [],
        };
    }
    const isInCart = (lock: DisplayLock): boolean =>
        actions.find((cartAction: BatchAction): boolean => {
            const data = extractDeleteActionData(cartAction);
            if (!data) {
                return false;
            }
            return (
                lock.lockId === data.lockId &&
                lock.team === data.team &&
                lock.application === data.app &&
                lock.environment === data.env
            );
        }) !== undefined;

    const resultLocks: AllLocks = {
        environmentLocks: [],
        appLocks: [],
        teamLocks: [],
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
    allLocks.teamLocks.forEach((teamLock: DisplayLock) => {
        if (isInCart(teamLock)) {
            return;
        }
        // if the id is the same, but we are on a different environment or different team:
        if (teamLock.lockId === data.lockId && (teamLock.environment !== data.env || teamLock.team !== data.team)) {
            resultLocks.teamLocks.push(teamLock);
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
export const useRelease = (application: string, version: number): Release | undefined => {
    const appDetails = useAppDetailsForApp(application);

    if (!appDetails || appDetails.appDetailState !== AppDetailsState.READY) return undefined;

    return appDetails.details ? appDetails.details.application?.releases.find((r) => r.version === version) : undefined;
};

export const useReleaseOrLog = (application: string, version: number): Release | undefined => {
    const release = useRelease(application, version);
    if (!release) {
        // eslint-disable-next-line no-console
        console.error('Release cannot be found for app ' + application + ' version ' + version);
        return undefined;
    }
    return release;
};

export const useReleaseOptional = (application: string, env: Environment): Release | undefined => {
    const response = useAppDetailsForApp(application);
    const appDetails = response.details;
    if (appDetails === undefined) {
        return undefined;
    }
    const deployment = appDetails.deployments[env.name];
    if (!deployment) return undefined;
    return appDetails.application?.releases.find((r) => r.version === deployment.version);
};

export type EnvironmentGroupExtended = EnvironmentGroup & { numberOfEnvsInGroup: number };

/**
 * returns the environments where a release is currently deployed
 */
export const useCurrentlyDeployedAtGroup = (application: string, version: number): EnvironmentGroupExtended[] => {
    const environmentGroups: EnvironmentGroup[] = useEnvironmentGroups();
    const response = useAppDetailsForApp(application);
    const appDetails = response.details;
    return useMemo(() => {
        const envGroups: EnvironmentGroupExtended[] = [];
        environmentGroups.forEach((group: EnvironmentGroup) => {
            const envs = group.environments.filter(
                (env) =>
                    appDetails &&
                    appDetails.deployments[env.name] &&
                    appDetails.deployments[env.name].version === version
            );
            if (envs.length > 0) {
                // we need to make a copy of the group here, because we want to remove some envs.
                // but that should not have any effect on the group saved in the store.
                const groupCopy: EnvironmentGroupExtended = {
                    environmentGroupName: group.environmentGroupName,
                    environments: envs,
                    distanceToUpstream: group.distanceToUpstream,
                    numberOfEnvsInGroup: group.environments.length,
                    priority: group.priority,
                };
                envGroups.push(groupCopy);
            }
        });
        return envGroups;
    }, [environmentGroups, version, appDetails]);
};

/**
 * returns the environments where an application is currently deployed
 */
export const useCurrentlyExistsAtGroup = (application: string): EnvironmentGroupExtended[] => {
    const environmentGroups: EnvironmentGroup[] = useEnvironmentGroups();
    const response = useAppDetailsForApp(application);
    const appDetails = response.details;

    return useMemo(() => {
        const envGroups: EnvironmentGroupExtended[] = [];
        environmentGroups.forEach((group: EnvironmentGroup) => {
            const envs = group.environments.filter((env) => (appDetails ? appDetails.deployments[env.name] : false));
            if (envs.length > 0) {
                // we need to make a copy of the group here, because we want to remove some envs.
                // but that should not have any effect on the group saved in the store.
                const groupCopy: EnvironmentGroupExtended = {
                    environmentGroupName: group.environmentGroupName,
                    environments: envs,
                    distanceToUpstream: group.distanceToUpstream,
                    numberOfEnvsInGroup: group.environments.length,
                    priority: group.priority,
                };
                envGroups.push(groupCopy);
            }
        });
        return envGroups;
    }, [environmentGroups, appDetails]);
};

// Calculated release difference between a specific release and currently deployed release on a specific environment
export const useReleaseDifference = (toDeployVersion: number, application: string, environment: string): number => {
    const response = useAppDetailsForApp(application);

    if (!response || !response.details) {
        return 0;
    }
    const appDetails = response.details;
    const deployment = appDetails.deployments[environment];
    if (!deployment) {
        return 0;
    }
    const currentDeployedIndex = appDetails.application?.releases.findIndex(
        (rel) => rel.version === deployment.version
    );
    const newVersionIndex = appDetails.application?.releases?.findIndex((rel) => rel.version === toDeployVersion);
    if (
        currentDeployedIndex === undefined ||
        newVersionIndex === undefined ||
        currentDeployedIndex === -1 ||
        newVersionIndex === -1
    ) {
        return 0;
    }

    return newVersionIndex - currentDeployedIndex;
};
// Get all minor releases for an app
export const useMinorsForApp = (app: string): number[] | undefined =>
    useAppDetailsForApp(app)
        .details?.application?.releases.filter((rel) => rel.isMinor)
        .map((rel) => rel.version);

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
    dexAuthEnabled: boolean;
    overviewLoaded: boolean;
};

// returns one loading state for all the calls done on startup, in order to render a spinner with details
export const useGlobalLoadingState = (): React.ReactElement | undefined => {
    const { configs, configReady } = useFrontendConfig((c) => c);
    const isAuthenticated = useIsAuthenticated();
    const azureAuthEnabled = configs.authConfig?.azureAuth?.enabled || false;
    const dexAuthEnabled = configs.authConfig?.dexAuth?.enabled || false;
    const overviewLoaded = useOverviewLoaded();
    const everythingLoaded = overviewLoaded && configReady && (isAuthenticated || !azureAuthEnabled);
    if (!configReady) {
        return (
            <LoadingStateSpinner
                loadingState={{
                    configReady,
                    isAuthenticated,
                    azureAuthEnabled,
                    dexAuthEnabled,
                    overviewLoaded,
                }}
            />
        );
    }

    const validToken = isTokenValid();
    if (dexAuthEnabled && !validToken) {
        return <LoginPage />;
    }

    if (!everythingLoaded) {
        return (
            <LoadingStateSpinner
                loadingState={{
                    configReady,
                    isAuthenticated,
                    azureAuthEnabled,
                    dexAuthEnabled,
                    overviewLoaded,
                }}
            />
        );
    }
    return undefined;
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

type RolloutStatusStore = {
    enabled: boolean;
    applications: {
        [application: string]: RolloutStatusApplication;
    };
};

const [useEntireRolloutStatus, rolloutStatus] = createStore<RolloutStatusStore>({ enabled: false, applications: {} });
class RolloutStatusGetter {
    private readonly store: RolloutStatusStore;

    constructor(store: RolloutStatusStore) {
        this.store = store;
    }

    getAppStatus(
        application: string,
        applicationVersion: number | undefined,
        environment: string
    ): RolloutStatus | undefined {
        if (!this.store.enabled) {
            return undefined;
        }
        const statusPerEnv = this.store.applications[application];
        if (statusPerEnv === undefined) {
            return undefined;
        }
        const status = statusPerEnv[environment];
        if (status === undefined) {
            return undefined;
        }
        if (status.rolloutStatus === RolloutStatus.ROLLOUT_STATUS_SUCCESFUL && status.version !== applicationVersion) {
            // The rollout service might be sligthly behind the UI.
            return RolloutStatus.ROLLOUT_STATUS_PENDING;
        }
        return status.rolloutStatus;
    }
}

export const useRolloutStatus = <T,>(f: (getter: RolloutStatusGetter) => T): T =>
    useEntireRolloutStatus((data) => f(new RolloutStatusGetter(data)));

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
export const invalidateAppDetailsForApp = (appName: string): void => {
    const details = updateAppDetails.get();
    details[appName] = {
        appDetailState: AppDetailsState.NOTREQUESTED,
        details: undefined,
        updatedAt: undefined,
        errorMessage: undefined,
    };
    updateAppDetails.set(details);
};

export const EnableRolloutStatus = (): void => {
    rolloutStatus.set({ enabled: true });
};

export const FlushRolloutStatus = (): void => {
    rolloutStatus.set({ enabled: false, applications: {} });
};

export const FlushGitSyncStatus = (): void => {
    gitSyncStatus.set({ enabled: false, sync_failed: [], unsyced: [] });
};

export const GetEnvironmentConfigPretty = (environmentName: string): Promise<string> =>
    useApi
        .environmentService()
        .GetEnvironmentConfig({ environment: environmentName })
        .then((res: GetEnvironmentConfigResponse) => {
            if (!res.config) {
                return Promise.reject(new Error('empty response.'));
            }
            return JSON.stringify(res.config, null, ' ');
        });

export const useArgoCDNamespace = (): string | undefined => useFrontendConfig((c) => c.configs.argoCd?.namespace);

type GitSyncStatusStore = {
    enabled: boolean;
    unsyced: EnvApp[];
    sync_failed: EnvApp[];
};

export const useGitSyncStatus = <T,>(f: (getter: GitSyncStatusGetter) => T): T =>
    useEntireGitSyncStatus((data) => f(new GitSyncStatusGetter(data)));
export const [useEntireGitSyncStatus, gitSyncStatus] = createStore<GitSyncStatusStore>({
    enabled: false,
    sync_failed: [],
    unsyced: [],
});

export const UpdateGitSyncStatus = (ev: GetGitSyncStatusResponse): void => {
    gitSyncStatus.set({
        enabled: true,
        unsyced: ev.unsynced,
        sync_failed: ev.syncFailed,
    });
};

export const EnableGitSyncStatus = (): void => {
    gitSyncStatus.set({ enabled: true });
};

class GitSyncStatusGetter {
    private readonly store: GitSyncStatusStore;

    constructor(store: GitSyncStatusStore) {
        this.store = store;
    }

    isEnabled(): boolean {
        return this.store.enabled;
    }

    getAppStatus(application: string, environment: string): number | undefined {
        if (!this.store.enabled) {
            return undefined;
        }

        let status = this.store.unsyced.find(
            (val) => val.applicationName === application && val.environmentName === environment
        );
        if (status) {
            return GitSyncStatus.GIT_SYNC_STATUS_SYNCING;
        }
        status = this.store.sync_failed.find(
            (val) => val.applicationName === application && val.environmentName === environment
        );
        if (status) {
            return GitSyncStatus.GIT_SYNC_STATUS_SYNC_ERROR;
        }
        return GitSyncStatus.GIT_SYNC_STATUS_STATUS_SUCCESSFULL;
    }
}
