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
import * as React from 'react';
import DeleteForeverIcon from '@material-ui/icons/DeleteForever';
import DeleteOutlineIcon from '@material-ui/icons/DeleteOutline';
import type { Environment, Release, BatchAction } from '../api/api';
import { Tooltip } from '@material-ui/core';
import { useMemo } from 'react';
import { ConfirmationDialogProvider } from './Batch';
import IconButton from '@material-ui/core/IconButton';

export enum DeployState {
    Normal,
    Undeploy,
    Mixed,
}

export function getDeployState(name: string, environments: { [name: string]: Environment }): DeployState {
    let allNormal = true;
    let allUndeploy = true;
    for (const envName in environments) {
        const application = environments[envName].applications[name];
        if (application) {
            if (application.undeployVersion) {
                allNormal = false;
            } else {
                allUndeploy = false;
            }
        }
    }
    if (allNormal) {
        return DeployState.Normal;
    }
    if (allUndeploy) {
        return DeployState.Undeploy;
    }
    return DeployState.Mixed;
}

export const UndeployBtn = (props: {
    openDialog?: () => void; //
    state?: string; //
    applicationName: string; //
}) => {
    const tooltip = 'This app is ready to un-deploy.';
    const btn = (btnProps: { onClick?: () => void | undefined }, disabled: boolean) => (
        <div className={'warning-prepare-undeploy-done'} {...btnProps}>
            <IconButton disabled={disabled}>
                <Tooltip title={tooltip} arrow={true}>
                    <DeleteForeverIcon color={'primary'} />
                </Tooltip>
            </IconButton>
        </div>
    );
    switch (props.state) {
        case 'not-in-cart':
            return btn({ onClick: props.openDialog }, false);
        case 'in-cart':
            return btn({}, true);
    }
    return null;
};

const UndeployRunningWarning: React.FC<any> = (props: { deployState: DeployState; name: string }) => {
    const tooltip = 'This app is in the process of deletion';
    const undeployHint = (
        <div className={'warning-undeploy-running'} title={tooltip}>
            <IconButton disabled>
                <DeleteOutlineIcon />
            </IconButton>
        </div>
    );
    const undeploy: BatchAction = useMemo(
        () => ({
            action: {
                $case: 'undeploy',
                undeploy: {
                    application: props.name,
                },
            },
        }),
        [props.name]
    );
    const Undeploy = (
        <ConfirmationDialogProvider action={undeploy}>
            <UndeployBtn applicationName={props.name} />
        </ConfirmationDialogProvider>
    );
    switch (props.deployState) {
        case DeployState.Normal:
            return null;
        case DeployState.Undeploy:
            return Undeploy;
        case DeployState.Mixed:
            return undeployHint;
    }
};

function isInconsistent(releases: Release[]): boolean {
    if (!releases || releases.length <= 1) {
        return false;
    }
    const currentReleaseUndeploy = releases[0].undeployVersion;
    const priorReleaseUndeploy = releases[1].undeployVersion;
    // if there was an "undeploy" version in the past, but now we have a normal version, we consider that "inconsistent"
    return priorReleaseUndeploy && !currentReleaseUndeploy;
}

const UndeployInconsistencyWarning: React.FC<any> = () => {
    const tooltip = 'Deletion of this app was interrupted.';
    return (
        <div className={'warning-inconsistent'} title={tooltip}>
            <IconButton disabled>
                <DeleteForeverIcon color={'error'} />
            </IconButton>
        </div>
    );
};

export type WarningsProps = {
    name: string;
    environments: { [name: string]: Environment };
    releases: Release[];
};

export const Warnings: React.FC<any> = (props: WarningsProps) => {
    if (isInconsistent(props.releases)) {
        return <UndeployInconsistencyWarning />;
    }
    const deployState = getDeployState(props.name, props.environments);
    return <UndeployRunningWarning deployState={deployState} name={props.name} />;
};
