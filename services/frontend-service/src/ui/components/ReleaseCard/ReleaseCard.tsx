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
import classNames from 'classnames';
import React from 'react';
import {
    useCurrentlyDeployedAtGroup,
    useOpenReleaseDialog,
    useRolloutStatus,
    EnvironmentGroupExtended,
    useReleaseOrLog,
    useAppDetailsForApp,
    useGitSyncStatus,
} from '../../utils/store';
import { Tooltip } from '../tooltip/tooltip';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { RolloutStatus } from '../../../api/api';
import { ReleaseVersion } from '../ReleaseVersion/ReleaseVersion';
import { RolloutStatusDescription } from '../RolloutStatusDescription/RolloutStatusDescription';
import { GitSyncStatus, GitSyncStatusDescription } from '../GitSyncStatusDescription/GitSyncStatusDescription';
import { Git, Argo } from '../../../images';

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

const RolloutStatusIcon: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            return <span className="rollout__icon_successful">✓</span>;
        case RolloutStatus.ROLLOUT_STATUS_PROGRESSING:
            return <span className="rollout__icon_progressing">↻</span>;
        case RolloutStatus.ROLLOUT_STATUS_PENDING:
            return <span className="rollout__icon_pending">⧖</span>;
        case RolloutStatus.ROLLOUT_STATUS_ERROR:
            return <span className="rollout__icon_error">!</span>;
        case RolloutStatus.ROLLOUT_STATUS_UNHEALTHY:
            return <span className="rollout__icon_unhealthy">⚠</span>;
    }
    return <span className="rollout__icon_unknown">?</span>;
};

const GitSyncStatusIcon: React.FC<{ status: GitSyncStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case GitSyncStatus.GIT_SYNC_STATUS_STATUS_SUCCESSFULL:
            return <span className="rollout__icon_successful">✓</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCING:
            return <span className="rollout__icon_progressing">↻</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_STATUS_SYNC_ERROR:
            return <span className="rollout__icon_error">!</span>;
    }
    return <span className="rollout__icon_unknown">?</span>;
};

// note that the order is important here.
// "most interesting" must come first.
// see `calculateDeploymentStatus`
// The same priority list is also implemented in pkg/service/broadcast.go.
const rolloutStatusPriority = [
    // Error is not recoverable by waiting and requires manual intervention
    RolloutStatus.ROLLOUT_STATUS_ERROR,

    // These states may resolve by waiting longer
    RolloutStatus.ROLLOUT_STATUS_PROGRESSING,
    RolloutStatus.ROLLOUT_STATUS_UNHEALTHY,
    RolloutStatus.ROLLOUT_STATUS_PENDING,
    RolloutStatus.ROLLOUT_STATUS_UNKNOWN,

    // This is the only successful state
    RolloutStatus.ROLLOUT_STATUS_SUCCESFUL,
];

const gitSyncStatusPriority = [
    GitSyncStatus.GIT_SYNC_STATUS_STATUS_SYNC_ERROR,
    GitSyncStatus.GIT_SYNC_STATUS_SYNCING,
    GitSyncStatus.GIT_SYNC_STATUS_STATUS_SUCCESSFULL,
];

const getStatusPriority = (status: number, priorities: number[]): number => {
    const idx = priorities.indexOf(status);
    if (idx === -1) {
        return priorities.length;
    }
    return idx;
};

type DeploymentStatus = {
    environmentGroup: string;
    rolloutStatus: RolloutStatus;
};

type DeploymentGitSyncStatus = {
    environmentGroup: string;
    gitSyncStatus: number; //status
};

const useDeploymentStatus = (
    app: string,
    deployedAt: EnvironmentGroupExtended[]
): [Array<DeploymentStatus>, RolloutStatus?] => {
    const appDetails = useAppDetailsForApp(app);
    const rolloutEnvGroups = useRolloutStatus((getter) => {
        const groups: { [envGroup: string]: RolloutStatus } = {};
        deployedAt.forEach((envGroup) => {
            const status = envGroup.environments.reduce((cur: RolloutStatus | undefined, env) => {
                const appVersion = appDetails.details?.deployments[env.name].version;
                const status = getter.getAppStatus(app, appVersion, env.name);
                if (cur === undefined) {
                    return status;
                }
                if (status === undefined) {
                    return cur;
                }
                if (getStatusPriority(status, rolloutStatusPriority) < getStatusPriority(cur, rolloutStatusPriority)) {
                    return status;
                }
                return cur;
            }, undefined);
            groups[envGroup.environmentGroupName] = status ?? RolloutStatus.ROLLOUT_STATUS_UNKNOWN;
        });
        return groups;
    });
    const rolloutEnvGroupsArray = Object.entries(rolloutEnvGroups).map((e) => ({
        environmentGroup: e[0],
        rolloutStatus: e[1],
    }));
    rolloutEnvGroupsArray.sort((a, b) => {
        if (a.environmentGroup < b.environmentGroup) {
            return -1;
        } else if (a.environmentGroup > b.environmentGroup) {
            return 1;
        }
        return 0;
    });
    // Calculates the most interesting rollout status according to the `rolloutStatusPriority`.
    const mostInteresting = rolloutEnvGroupsArray.reduce(
        (cur: RolloutStatus | undefined, item) =>
            cur === undefined
                ? item.rolloutStatus
                : getStatusPriority(item.rolloutStatus, rolloutStatusPriority) <
                    getStatusPriority(cur, rolloutStatusPriority)
                  ? item.rolloutStatus
                  : cur,
        undefined
    );
    return [rolloutEnvGroupsArray, mostInteresting];
};

const useSyncStatusForDeployment = (
    app: string,
    deployedAt: EnvironmentGroupExtended[]
): [Array<DeploymentGitSyncStatus>, number?] => {
    const rolloutEnvGroups = useGitSyncStatus((getter) => {
        const groups: { [envGroup: string]: number } = {};
        deployedAt.forEach((envGroup) => {
            const status = envGroup.environments.reduce((cur: number | undefined, env) => {
                const status = getter.getAppStatus(app, env.name);
                if (cur === undefined) {
                    return status;
                }
                if (status === undefined) {
                    return cur;
                }
                if (getStatusPriority(status, gitSyncStatusPriority) < getStatusPriority(cur, gitSyncStatusPriority)) {
                    return status;
                }
                return cur;
            }, undefined);
            groups[envGroup.environmentGroupName] = status ?? GitSyncStatus.GIT_SYNC_STATUS_UNRECOGNIZED;
        });
        return groups;
    });

    const gitSyncStatusEnvGroupsArray = Object.entries(rolloutEnvGroups).map((e) => ({
        environmentGroup: e[0],
        gitSyncStatus: e[1],
    }));
    gitSyncStatusEnvGroupsArray.sort((a, b) => {
        if (a.environmentGroup < b.environmentGroup) {
            return -1;
        } else if (a.environmentGroup > b.environmentGroup) {
            return 1;
        }
        return 0;
    });
    // Calculates the most interesting rollout status according to the `rolloutStatusPriority`.
    const mostInteresting = gitSyncStatusEnvGroupsArray.reduce(
        (cur: GitSyncStatus | undefined, item) =>
            cur === undefined
                ? item.gitSyncStatus
                : getStatusPriority(item.gitSyncStatus, gitSyncStatusPriority) <
                    getStatusPriority(cur, gitSyncStatusPriority)
                  ? item.gitSyncStatus
                  : cur,
        undefined
    );

    return [gitSyncStatusEnvGroupsArray, mostInteresting];
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const openReleaseDialog = useOpenReleaseDialog(app, version);
    const deployedAt = useCurrentlyDeployedAtGroup(app, version);

    const syncStatus = useGitSyncStatus((getter) => getter);

    const [rolloutEnvs, mostInteresting] = useDeploymentStatus(app, deployedAt);
    const [gitSyncStatuses, mostInterestingSyncStatus] = useSyncStatusForDeployment(app, deployedAt);
    const release = useReleaseOrLog(app, version);
    if (!release) {
        return null;
    }
    const { createdAt, sourceMessage, sourceAuthor, undeployVersion, isMinor, isPrepublish } = release;

    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {!!sourceMessage && (
                <b>
                    {sourceMessage} {isMinor ? '💤' : ''}
                </b>
            )}
            {!!sourceAuthor && (
                <div>
                    <span>Author:</span> {sourceAuthor}
                </div>
            )}
            {isMinor && (
                <div>
                    <span>
                        '💤' icon means that this release is a minor release; it has no changes to the manifests
                        comparing to the previous release.
                    </span>
                </div>
            )}
            {isPrepublish && (
                <div className="prerelease__description">
                    <span>This is a pre-release. It doesn't have any manifests. It can't be deployed anywhere.</span>
                </div>
            )}
            {!!createdAt && (
                <div className="release__metadata">
                    <span>Created </span>
                    <FormattedDate className={'date'} createdAt={createdAt} />
                </div>
            )}
            {rolloutEnvs.length > 0 && (
                <table className="release__environment_status">
                    <thead>
                        <tr>
                            <th>Environment group</th>

                            {syncStatus.isEnabled() ? (
                                <th className="release-card__statusth">
                                    Sync Status
                                    <Git className="status-logo" />
                                </th>
                            ) : (
                                <th className="release-card__statusth">
                                    Rollout Status <Argo className="status-logo" />
                                </th>
                            )}
                        </tr>
                    </thead>
                    <tbody>
                        {syncStatus.isEnabled()
                            ? gitSyncStatuses.map((env) => (
                                  <tr key={env.environmentGroup}>
                                      <td>{env.environmentGroup}</td>
                                      <td>
                                          <GitSyncStatusDescription
                                              status={env.gitSyncStatus}></GitSyncStatusDescription>
                                      </td>
                                  </tr>
                              ))
                            : rolloutEnvs.map((env) => (
                                  <tr key={env.environmentGroup}>
                                      <td>{env.environmentGroup}</td>
                                      <td>
                                          <RolloutStatusDescription status={env.rolloutStatus} />
                                      </td>
                                  </tr>
                              ))}
                    </tbody>
                </table>
            )}
        </div>
    );

    const firstLine = (isMinor ? '💤' : '') + sourceMessage.split('\n')[0];
    return (
        <Tooltip id={app + version} tooltipContent={tooltipContents}>
            <div className={'release-card__container'}>
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} smallEnvChip />
                </div>
                <div
                    className={classNames(
                        'mdc-card release-card',
                        className,
                        release.isPrepublish ? 'release-card__prepublish' : ''
                    )}>
                    <div
                        className="mdc-card__primary-action release-card__description"
                        tabIndex={0}
                        onClick={openReleaseDialog}>
                        <div className="release-card__header">
                            <div
                                className={classNames(
                                    'release__title',
                                    release.isPrepublish ? 'release__title__prepublish' : ''
                                )}>
                                {undeployVersion ? 'Undeploy Version' : firstLine}
                            </div>
                            <ReleaseVersion release={release} />
                        </div>
                        {syncStatus.isEnabled()
                            ? mostInterestingSyncStatus !== undefined && (
                                  <div className="release__status">
                                      <GitSyncStatusIcon status={mostInterestingSyncStatus} />
                                  </div>
                              )
                            : mostInteresting !== undefined && (
                                  <div className="release__status">
                                      <RolloutStatusIcon status={mostInteresting} />
                                  </div>
                              )}
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </div>
        </Tooltip>
    );
};
