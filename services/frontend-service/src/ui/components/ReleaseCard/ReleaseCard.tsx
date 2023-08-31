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
import { Button } from '../button';
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

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

const RolloutStatusIcon: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case RolloutStatus.RolloutStatusSuccesful:
            return <span className="rollout__icon_successful">✓</span>;
        case RolloutStatus.RolloutStatusProgressing:
            return <span className="rollout__icon_progressing">↻</span>;
        case RolloutStatus.RolloutStatusError:
            return <span className="rollout__icon_error">!</span>;
    }
    return <span className="rollout__icon_unknown">?</span>;
};

const RolloutStatusDescription: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case RolloutStatus.RolloutStatusSuccesful:
            return <span className="rollout__description_successful">✓ Done</span>;
        case RolloutStatus.RolloutStatusProgressing:
            return <span className="rollout__description_progressing">↻ In progress</span>;
        case RolloutStatus.RolloutStatusError:
            return <span className="rollout__description_error">! Failed</span>;
    }
    return <span className="rollout__description_unknown">? Unkwown</span>;
};

// note that the order is important here.
// "most interesting" must come first.
// see `calculateDeploymentStatus`
const rolloutStatusPriority = [
    RolloutStatus.RolloutStatusError,
    RolloutStatus.RolloutStatusProgressing,
    RolloutStatus.RolloutStatusUnknown,
];

const calculateDeploymentStatus = (
    app: string,
    deployedAt: EnvironmentGroupExtended[],
    rolloutStatus: RolloutStatusApplication
): [Array<StreamStatusResponse>, StreamStatusResponse?] => {
    const rolloutEnvs = deployedAt.flatMap((envGroup) =>
        envGroup.environments.map((env) => {
            const status = rolloutStatus[env.name];
            if (!status) {
                // The status is completly unknown. Either the app is just new or rollout service is not responding.
                return {
                    environment: env.name,
                    rolloutStatus: RolloutStatus.RolloutStatusUnknown,
                    version: 0,
                    application: app,
                };
            }
            if (
                status.rolloutStatus === RolloutStatus.RolloutStatusSuccesful &&
                status.version !== env.applications[app]?.version
            ) {
                // The rollout service might be sligthly behind the UI.
                // In that case the
                return { ...status, rolloutStatus: RolloutStatus.RolloutStatusProgressing };
            }
            return status;
        })
    );
    rolloutEnvs.sort((a, b) => {
        if (a.environment < b.environment) {
            return -1;
        } else if (a.environment > b.environment) {
            return 1;
        }
        return 0;
    });
    const mostInteresting = [...rolloutEnvs].sort((a, b) => {
        const aPriority = rolloutStatusPriority.indexOf(a.rolloutStatus) ?? rolloutStatusPriority.length;
        const bPriority = rolloutStatusPriority.indexOf(b.rolloutStatus) ?? rolloutStatusPriority.length;
        return aPriority - bPriority;
    })[0];
    return [rolloutEnvs, mostInteresting];
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const { createdAt, sourceMessage, sourceCommitId, sourceAuthor, undeployVersion } = useReleaseOrThrow(app, version);
    const openReleaseDialog = useOpenReleaseDialog(app, version);
    const rolloutStatus = useRolloutStatus(app);
    const deployedAt = useCurrentlyDeployedAtGroup(app, version);

    const [rolloutEnvs, mostInteresting] = calculateDeploymentStatus(app, deployedAt, rolloutStatus);

    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {!!sourceMessage && <b>{sourceMessage}</b>}
            {!!sourceCommitId && (
                <div className={'release__hash--container'}>
                    <Button className="release__hash" label={'' + sourceCommitId} />
                </div>
            )}
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
                            <th>Environment</th>
                            <th>Rollout</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rolloutEnvs.map((env) => (
                            <tr key={env.environment}>
                                <td>{env.environment}</td>
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
                            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
                        </div>
                        {mostInteresting && (
                            <div className="release__status">
                                <RolloutStatusIcon status={mostInteresting.rolloutStatus} />
                            </div>
                        )}
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </div>
        </Tooltip>
    );
};
