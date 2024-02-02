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
import classNames from 'classnames';
import React from 'react';
import {
    useCurrentlyDeployedAtGroup,
    useOpenReleaseDialog,
    useReleaseOrThrow,
    useRolloutStatus,
    RolloutStatusApplication,
    EnvironmentGroupExtended,
} from '../../utils/store';
import { Tooltip } from '../tooltip/tooltip';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { RolloutStatus, StreamStatusResponse } from '../../../api/api';
import { ReleaseVersion } from '../ReleaseVersion/ReleaseVersion';
import { RolloutStatusDescription } from '../RolloutStatusDescription/RolloutStatusDescription';

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

const getRolloutStatusPriority = (status: RolloutStatus): number => {
    const idx = rolloutStatusPriority.indexOf(status);
    if (idx === -1) {
        return rolloutStatusPriority.length;
    }
    return idx;
};

type DeploymentStatus = {
    environmentGroup: string;
    rolloutStatus: RolloutStatus;
};

const getAppRolloutStatus = (
    status: StreamStatusResponse | undefined,
    deployedVersion: number | undefined
): RolloutStatus => {
    if (status === undefined) {
        // The status is completly unknown. Either the app is just new or rollout service is not responding.
        return RolloutStatus.ROLLOUT_STATUS_UNKNOWN;
    }
    if (status.rolloutStatus === RolloutStatus.ROLLOUT_STATUS_SUCCESFUL && status.version !== deployedVersion) {
        // The rollout service might be sligthly behind the UI.
        return RolloutStatus.ROLLOUT_STATUS_PENDING;
    }
    return status.rolloutStatus;
};

const calculateDeploymentStatus = (
    app: string,
    deployedAt: EnvironmentGroupExtended[],
    rolloutEnabled: boolean,
    rolloutStatus: RolloutStatusApplication
): [Array<DeploymentStatus>, RolloutStatus?] => {
    if (!rolloutEnabled) {
        return [[], undefined];
    }
    const rolloutEnvGroups = deployedAt.map((envGroup) => {
        const status = envGroup.environments.reduce((cur: RolloutStatus | undefined, env) => {
            const appVersion: number | undefined = env.applications[app]?.version;
            const status = getAppRolloutStatus(rolloutStatus[env.name], appVersion);
            if (cur === undefined) {
                return status;
            }
            if (getRolloutStatusPriority(status) < getRolloutStatusPriority(cur)) {
                return status;
            }
            return cur;
        }, undefined);
        return {
            environmentGroup: envGroup.environmentGroupName,
            rolloutStatus: status ?? RolloutStatus.ROLLOUT_STATUS_UNKNOWN,
        };
    });
    rolloutEnvGroups.sort((a, b) => {
        if (a.environmentGroup < b.environmentGroup) {
            return -1;
        } else if (a.environmentGroup > b.environmentGroup) {
            return 1;
        }
        return 0;
    });
    // Calculates the most interesting rollout status according to the `rolloutStatusPriority`.
    const mostInteresting = rolloutEnvGroups.reduce(
        (cur: RolloutStatus | undefined, item) =>
            cur === undefined
                ? item.rolloutStatus
                : getRolloutStatusPriority(item.rolloutStatus) < getRolloutStatusPriority(cur)
                  ? item.rolloutStatus
                  : cur,
        undefined
    );
    return [rolloutEnvGroups, mostInteresting];
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const release = useReleaseOrThrow(app, version);
    const { createdAt, sourceMessage, sourceAuthor, undeployVersion } = release;
    const openReleaseDialog = useOpenReleaseDialog(app, version);
    const [rolloutEnabled, rolloutStatus] = useRolloutStatus(app);
    const deployedAt = useCurrentlyDeployedAtGroup(app, version);

    const [rolloutEnvs, mostInteresting] = calculateDeploymentStatus(app, deployedAt, rolloutEnabled, rolloutStatus);

    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {!!sourceMessage && <b>{sourceMessage}</b>}
            {!!sourceAuthor && (
                <div>
                    <span>Author:</span> {sourceAuthor}
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
                            <th>Rollout</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rolloutEnvs.map((env) => (
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

    const firstLine = sourceMessage.split('\n')[0];
    return (
        <Tooltip id={app + version} tooltipContent={tooltipContents}>
            <div className="release-card__container">
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} smallEnvChip />
                </div>
                <div className={classNames('mdc-card release-card', className)}>
                    <div
                        className="mdc-card__primary-action release-card__description"
                        // ref={control}
                        tabIndex={0}
                        onClick={openReleaseDialog}>
                        <div className="release-card__header">
                            <div className="release__title">{undeployVersion ? 'Undeploy Version' : firstLine}</div>
                            <ReleaseVersion release={release} />
                        </div>
                        {mostInteresting !== undefined && (
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
