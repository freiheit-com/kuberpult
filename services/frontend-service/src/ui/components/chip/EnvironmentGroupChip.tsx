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
import { EnvPrio } from '../ReleaseDialog/ReleaseDialog';
import { Environment } from '../../../api/api';
import React from 'react';
import { EnvironmentGroupExtended, useCurrentlyDeployedAtGroup } from '../../utils/store';
import { Tooltip } from '@material-ui/core';
import { Button } from '../button';
import { LocksWhite } from '../../../images';

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
    const priority = env.priority;
    const priorityClassName = className + '-' + String(EnvPrio[priority]).toLowerCase();
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
                <Tooltip
                    key={lock.lockId}
                    arrow
                    title={'Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}>
                    <div>
                        <Button
                            icon={<LocksWhite className="env-card-env-lock-icon" width="24px" height="24px" />}
                            className={'button-lock'}
                        />
                    </div>
                </Tooltip>
            ))}
        </div>
    ) : (
        !!Object.entries(env.locks).length && (
            <div className={classNames(className, 'env-locks')}>
                <LocksWhite className="env-card-env-lock-icon" width="18px" height="18px" />
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
