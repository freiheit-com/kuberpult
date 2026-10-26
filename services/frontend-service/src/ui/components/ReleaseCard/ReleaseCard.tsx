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
    EnvironmentGroupExtended,
    useReleaseOrLog,
    useGitSyncStatus,
    ReleaseNumbers,
} from '../../utils/store';
import { Tooltip } from '../tooltip/tooltip';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { GitSyncStatus } from '../../../api/api';
import { ReleaseVersion } from '../ReleaseVersion/ReleaseVersion';
import { GitSyncStatusDescription } from '../GitSyncStatusDescription/GitSyncStatusDescription';
import { Git } from '../../../images';

export type ReleaseCardProps = {
    className?: string;
    versionInfo: ReleaseNumbers;
    app: string;
};

const GitSyncStatusIcon: React.FC<{ status: GitSyncStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCED:
            return <span className="rollout__icon_successful">✓</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED:
            return <span className="rollout__icon_progressing">↻</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_ERROR:
            return <span className="rollout__icon_error">!</span>;
    }
    return <span className="rollout__icon_unknown">?</span>;
};

const gitSyncStatusPriority = [
    GitSyncStatus.GIT_SYNC_STATUS_ERROR,
    GitSyncStatus.GIT_SYNC_STATUS_UNKNOWN,
    GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
    GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
];
const getStatusPriority = (status: number, priorities: number[]): number => {
    const idx = priorities.indexOf(status);
    if (idx === -1) {
        return priorities.length;
    }
    return idx;
};

type DeploymentGitSyncStatus = {
    environmentGroup: string;
    gitSyncStatus: number; //status
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
            groups[envGroup.environmentGroupName] = status ?? GitSyncStatus.GIT_SYNC_STATUS_UNKNOWN;
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
    const { className, app, versionInfo } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const openReleaseDialog = useOpenReleaseDialog(app, versionInfo.version, versionInfo.revision);
    const deployedAt = useCurrentlyDeployedAtGroup(app, versionInfo);

    const syncStatus = useGitSyncStatus((getter) => getter);

    const [gitSyncStatuses, mostInterestingSyncStatus] = useSyncStatusForDeployment(app, deployedAt);
    const release = useReleaseOrLog(app, versionInfo.version, versionInfo.revision);
    if (!release) {
        return null;
    }
    const { createdAt, sourceMessage, sourceAuthor, undeployVersion, isMinor, isPrepublish } = release;

    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {!!sourceMessage && (
                <b className={'tooltip-text'}>
                    {sourceMessage} {isMinor ? '💤' : ''}
                </b>
            )}
            {!!sourceAuthor && (
                <div className={'tooltip-text'}>
                    <span>Author:</span> {sourceAuthor}
                </div>
            )}
            {isMinor && (
                <div>
                    <span className={'tooltip-text'}>
                        '💤' icon means that this release is a minor release; it has no changes to the manifests
                        comparing to the previous release.
                    </span>
                </div>
            )}
            {isPrepublish && (
                <div className="prerelease__description tooltip-text">
                    <span>This is a pre-release. It doesn't have any manifests. It can't be deployed anywhere.</span>
                </div>
            )}
            {!!createdAt && (
                <div className="release__metadata tooltip-text">
                    <span>Created </span>
                    <FormattedDate className={'date'} createdAt={createdAt} />
                </div>
            )}
            {
                <table className="release__environment_status">
                    <thead>
                        <tr>
                            <th className={'tooltip-text'}>Environment group</th>

                            {syncStatus.isEnabled() ? (
                                <th className="release-card__statusth tooltip-text">
                                    Sync Status
                                    <Git className="status-logo" />
                                </th>
                            ) : null}
                        </tr>
                    </thead>
                    <tbody>
                        {syncStatus.isEnabled()
                            ? gitSyncStatuses.map((env) => (
                                  <tr key={env.environmentGroup}>
                                      <td className={'tooltip-text'}>{env.environmentGroup}</td>
                                      <td>
                                          <GitSyncStatusDescription
                                              status={env.gitSyncStatus}></GitSyncStatusDescription>
                                      </td>
                                  </tr>
                              ))
                            : null}
                    </tbody>
                </table>
            }
        </div>
    );

    const firstLine = (isMinor ? '💤' : '') + sourceMessage.split('\n')[0];
    return (
        <Tooltip id={app + versionInfo.version + versionInfo.revision} tooltipContent={tooltipContents}>
            <div className={'release-card__container'}>
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.versionInfo} smallEnvChip />
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
                            : null}
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </div>
        </Tooltip>
    );
};
