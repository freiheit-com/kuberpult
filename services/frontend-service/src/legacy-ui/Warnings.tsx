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
import * as React from 'react';
import DeleteForeverIcon from '@material-ui/icons/DeleteForever';
import DeleteOutlineIcon from '@material-ui/icons/DeleteOutline';
import type { Environment, Release } from '../api/api';
import { Tooltip } from '@material-ui/core';
import { useMemo } from 'react';
import { ConfirmationDialogProvider } from './ConfirmationDialog';
import IconButton from '@material-ui/core/IconButton';
import { CartAction } from './ActionDetails';

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
    addToCart?: () => void; //
    inCart?: boolean; //
    applicationName: string; //
}) => {
    const tooltip = 'This app is ready to un-deploy.';
    return (
        <IconButton className={'warning-prepare-undeploy-done'} disabled={props.inCart} onClick={props.addToCart}>
            <Tooltip title={tooltip} arrow={true}>
                <DeleteForeverIcon color={'primary'} />
            </Tooltip>
        </IconButton>
    );
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
    const undeploy: CartAction = useMemo(
        () => ({
            undeploy: {
                application: props.name,
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
