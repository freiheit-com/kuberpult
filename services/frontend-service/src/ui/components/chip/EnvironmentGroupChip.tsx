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
import classNames from 'classnames';
import { EnvPrio } from '../ReleaseDialog/ReleaseDialog';
import { Environment, EnvironmentGroup } from '../../../api/api';
import React from 'react';
import { EnvironmentGroupExtended, useCurrentlyDeployedAtGroup } from '../../utils/store';

export const EnvironmentChip = (props: {
    className: string;
    env: Environment;
    groupNameOverride?: string;
    numberEnvsDeployed?: number;
    numberEnvsInGroup?: number;
}) => {
    const { className, env } = props;
    const priority = env.priority;
    const prioClassName = className + '-' + String(EnvPrio[priority]).toLowerCase();
    const name = props.groupNameOverride ? props.groupNameOverride : env.name;
    const numberString =
        props.numberEnvsDeployed && props.numberEnvsInGroup
            ? '(' + props.numberEnvsDeployed + '/' + props.numberEnvsInGroup + ')'
            : '';
    return (
        <span className={classNames('mdc-evolution-chip', className, prioClassName)} role="row">
            <span
                className="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary ENV"
                role="gridcell">
                <span className="mdc-evolution-chip__text-label">
                    <span>
                        {name} {numberString}
                    </span>
                </span>
            </span>
        </span>
    );
};

export const EnvironmentGroupChip = (props: { className: string; envGroup: EnvironmentGroupExtended }) => {
    const { className, envGroup } = props;

    // we display it different if there's only one env in this group:
    const displayAsGroup = envGroup.environments.length >= 2;
    if (displayAsGroup) {
        return (
            <>
                <div className={'EnvironmentGroupChip'}>
                    <EnvironmentChip
                        className={className}
                        env={envGroup.environments[0]}
                        groupNameOverride={envGroup.environmentGroupName}
                        numberEnvsDeployed={envGroup.environments.length}
                        numberEnvsInGroup={envGroup.numberOfEnvsInGroup}
                    />
                </div>
            </>
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
        />
    );
};

export type EnvChipListProps = {
    version: number;
    app: string;
};

export const EnvironmentGroupChipList: React.FC<EnvChipListProps> = (props) => {
    // const envGroups = useEnvironmentGroups();
    const deployedAt = useCurrentlyDeployedAtGroup(props.app, props.version);
    return (
        <>
            {' '}
            {deployedAt.map((envGroup) => (
                <EnvironmentGroupChip
                    key={envGroup.environmentGroupName}
                    envGroup={envGroup}
                    className={'release-environment'}
                />
            ))}{' '}
        </>
    );
};
