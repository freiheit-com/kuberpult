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
import { Environment, Lock } from '../../../api/api';
import React, { useCallback } from 'react';
import {
    addAction,
    EnvironmentGroupExtended,
    getPriorityClassName,
    useCurrentlyDeployedAtGroup,
} from '../../utils/store';
import { Tooltip } from '@material-ui/core';
import { Button } from '../button';
import { LocksWhite } from '../../../images';

export const EnvLock: React.FC<{
    env: string;
    lock: Lock;
}> = ({ env, lock }) => {
    const deleteEnvLock = useCallback(() => {
        addAction({
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: { environment: env, lockId: lock.lockId },
            },
        });
    }, [env, lock.lockId]);
    return (
        <Tooltip
            key={lock.lockId}
            arrow
            title={'Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}>
            <div>
                <Button
                    icon={<LocksWhite className="env-card-env-lock-icon" width="16px" height="16px" />}
                    className={'button-lock'}
                    onClick={deleteEnvLock}
                />
            </div>
        </Tooltip>
    );
};

export const AppLockSummary: React.FC<{
    app: string;
    numLocks: number;
}> = ({ app, numLocks }) => {
    const plural = numLocks === 1 ? 'lock' : 'locks';
    return (
        <Tooltip
            key={'app-lock-hint-' + app}
            arrow
            title={'"' + app + '" has ' + numLocks + ' application ' + plural + '. Click on a tile to see details.'}>
            <div>
                &nbsp;
                <LocksWhite className="env-card-env-lock-icon" width="16px" height="16px" />
            </div>
        </Tooltip>
    );
};

export type EnvironmentChipProps = {
    className: string;
    env: Environment;
    groupNameOverride?: string;
    numberEnvsDeployed?: number;
    numberEnvsInGroup?: number;
    smallEnvChip?: boolean;
};

export const EnvironmentChip = (props: EnvironmentChipProps): JSX.Element => {
    const { className, env, smallEnvChip } = props;
    const priorityClassName = getPriorityClassName(env);
    const name = props.groupNameOverride ? props.groupNameOverride : env.name;
    const numberString =
        props.numberEnvsDeployed && props.numberEnvsInGroup
            ? props.numberEnvsDeployed !== props.numberEnvsInGroup
                ? '(' + props.numberEnvsDeployed + '/' + props.numberEnvsInGroup + ')'
                : '(' + props.numberEnvsInGroup + ')' // when all envs are deployed, only show the total number on envs
            : '';
    const locks = !smallEnvChip ? (
        <div className={classNames(className, 'env-locks')}>
            {Object.values(env.locks).map((lock) => (
                <EnvLock env={env.name} lock={lock} key={lock.lockId} />
            ))}
        </div>
    ) : (
        !!Object.entries(env.locks).length && (
            <div className={classNames(className, 'env-locks')}>
                <LocksWhite className="env-card-env-lock-icon" width="12px" height="12px" />
            </div>
        )
    );
    return (
        <div className={classNames('mdc-evolution-chip', className, priorityClassName)} role="row">
            <span
                className="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary"
                role="gridcell">
                <span className="mdc-evolution-chip__text-name">{smallEnvChip ? name[0].toUpperCase() : name}</span>{' '}
                <span className="mdc-evolution-chip__text-numbers">{numberString}</span>
                {locks}
            </span>
        </div>
    );
};

export const EnvironmentGroupChip = (props: {
    className: string;
    envGroup: EnvironmentGroupExtended;
    smallEnvChip?: boolean;
}): JSX.Element => {
    const { className, envGroup, smallEnvChip } = props;

    // we display it different if there's only one env in this group:
    const displayAsGroup = envGroup.environments.length >= 2;
    if (displayAsGroup) {
        return (
            <div className={'EnvironmentGroupChip'}>
                <EnvironmentChip
                    className={className}
                    env={envGroup.environments[0]}
                    groupNameOverride={envGroup.environmentGroupName}
                    numberEnvsDeployed={envGroup.environments.length}
                    numberEnvsInGroup={envGroup.numberOfEnvsInGroup}
                    smallEnvChip={smallEnvChip}
                />
            </div>
        );
    }
    // since there's only 1 env, we display that:
    return (
        <EnvironmentChip
            className={className}
            env={envGroup.environments[0]}
            groupNameOverride={undefined}
            numberEnvsDeployed={1}
            numberEnvsInGroup={envGroup.numberOfEnvsInGroup}
            smallEnvChip={smallEnvChip}
        />
    );
};

export type EnvChipListProps = {
    version: number;
    app: string;
    smallEnvChip?: boolean;
};

export const EnvironmentGroupChipList: React.FC<EnvChipListProps> = (props) => {
    const deployedAt = useCurrentlyDeployedAtGroup(props.app, props.version);
    return (
        <div className={'env-group-chip-list env-group-chip-list-test'}>
            {' '}
            {deployedAt.map((envGroup) => (
                <EnvironmentGroupChip
                    key={envGroup.environmentGroupName}
                    envGroup={envGroup}
                    className={'release-environment'}
                    smallEnvChip={props.smallEnvChip}
                />
            ))}{' '}
        </div>
    );
};
