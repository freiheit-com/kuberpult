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
import {
    DeleteForeverRounded,
    DeleteOutlineRounded,
    Error,
    LockOpenRounded,
    LockRounded,
    MoveToInboxRounded,
} from '@material-ui/icons';
import * as React from 'react';
import { BatchAction, LockBehavior } from '../api/api';

export enum ActionTypes {
    Deploy,
    PrepareUndeploy,
    Undeploy,
    CreateEnvironmentLock,
    DeleteEnvironmentLock,
    CreateApplicationLock,
    DeleteApplicationLock,
    UNKNOWN,
}

export type Deploy = {
    deploy: {
        environment: string;
        application: string;
        version: number;
    };
};

export type PrepareUndeploy = {
    prepareUndeploy: {
        application: string;
    };
};

export type Undeploy = {
    undeploy: {
        application: string;
    };
};

export type CreateEnvironmentLock = {
    createEnvironmentLock: {
        environment: string;
    };
};

export type CreateApplicationLock = {
    createApplicationLock: {
        environment: string;
        application: string;
    };
};

export type DeleteEnvironmentLock = {
    deleteEnvironmentLock: {
        environment: string;
        lockId: string;
    };
};

export type DeleteApplicationLock = {
    deleteApplicationLock: {
        environment: string;
        application: string;
        lockId: string;
    };
};

export type CartAction =
    | Deploy
    | PrepareUndeploy
    | Undeploy
    | CreateEnvironmentLock
    | DeleteEnvironmentLock
    | CreateApplicationLock
    | DeleteApplicationLock;

const lockActions = [ActionTypes.CreateEnvironmentLock, ActionTypes.CreateApplicationLock] as const;
type lockAction = typeof lockActions[number];
export const hasLockAction = (actions: CartAction[]) =>
    actions.some((act) => lockActions.includes(getActionDetails(act).type as lockAction));

const deployActions = [ActionTypes.Deploy, ActionTypes.Undeploy, ActionTypes.PrepareUndeploy] as const;
type deployAction = typeof deployActions[number];
export const isDeployAction = (act: CartAction) => deployActions.includes(getActionDetails(act).type as deployAction);

type ActionDetails = {
    type: ActionTypes;
    name: string;
    summary: string;
    dialogTitle: string;
    description?: string;
    icon: React.ReactElement;

    // action details optional
    environment?: string;
    application?: string;
    lockId?: string;
    lockMessage?: string;
    version?: number;
};
export const getActionDetails = (action: CartAction): ActionDetails => {
    if ('deploy' in action)
        return {
            type: ActionTypes.Deploy,
            name: 'Deploy',
            dialogTitle: 'Are you sure you want to deploy this version?',
            summary:
                'Deploy version ' +
                action.deploy.version +
                ' of "' +
                action.deploy.application +
                '" to ' +
                action.deploy.environment,
            icon: <MoveToInboxRounded />,
            environment: action.deploy.environment,
            application: action.deploy.application,
            version: action.deploy.version,
        };
    if ('createEnvironmentLock' in action)
        return {
            type: ActionTypes.CreateEnvironmentLock,
            name: 'Create Env Lock',
            dialogTitle: 'Are you sure you want to add this environment lock?',
            summary: 'Create new environment lock on ' + action.createEnvironmentLock.environment,
            icon: <LockRounded />,
            environment: action.createEnvironmentLock.environment,
        };
    if ('createApplicationLock' in action)
        return {
            type: ActionTypes.CreateApplicationLock,
            name: 'Create App Lock',
            dialogTitle: 'Are you sure you want to add this application lock?',
            summary:
                'Lock "' +
                action.createApplicationLock.application +
                '" on ' +
                action.createApplicationLock.environment,
            icon: <LockRounded />,
            environment: action.createApplicationLock.environment,
            application: action.createApplicationLock.application,
        };
    if ('deleteEnvironmentLock' in action)
        return {
            type: ActionTypes.DeleteEnvironmentLock,
            name: 'Delete Env Lock',
            dialogTitle: 'Are you sure you want to delete this environment lock?',
            summary: 'Delete environment lock on ' + action.deleteEnvironmentLock.environment,
            icon: <LockOpenRounded />,
            environment: action.deleteEnvironmentLock.environment,
            lockId: action.deleteEnvironmentLock.lockId,
        };
    if ('deleteApplicationLock' in action)
        return {
            type: ActionTypes.DeleteApplicationLock,
            name: 'Delete App Lock',
            dialogTitle: 'Are you sure you want to delete this application lock?',
            summary:
                'Unlock "' +
                action.deleteApplicationLock.application +
                '" on ' +
                action.deleteApplicationLock.environment,
            icon: <LockOpenRounded />,
            environment: action.deleteApplicationLock.environment,
            application: action.deleteApplicationLock.application,
            lockId: action.deleteApplicationLock.lockId,
        };
    if ('prepareUndeploy' in action)
        return {
            type: ActionTypes.PrepareUndeploy,
            name: 'Prepare Undeploy',
            dialogTitle: 'Are you sure you want to start undeploy?',
            description:
                'The new version will go through the same cycle as any other versions' +
                ' (e.g. development->staging->production). ' +
                'The behavior is similar to any other version that is created normally.',
            summary: 'Prepare undeploy version for Application ' + action.prepareUndeploy.application,
            icon: <DeleteOutlineRounded />,
            application: action.prepareUndeploy.application,
        };
    if ('undeploy' in action)
        return {
            type: ActionTypes.Undeploy,
            name: 'Undeploy',
            dialogTitle: 'Are you sure you want to undeploy this application?',
            description: 'This application will be deleted permanently',
            summary: 'Undeploy and delete Application ' + action.undeploy.application,
            icon: <DeleteForeverRounded />,
            application: action.undeploy.application,
        };
    else
        return {
            type: ActionTypes.UNKNOWN,
            name: 'invalid',
            dialogTitle: 'invalid',
            summary: 'invalid',
            icon: <Error />,
        };
};

const randomLockId = () => 'ui-' + Math.random().toString(36).substring(7);

export const transformToBatch = (act: CartAction, m: string): BatchAction => {
    if ('createEnvironmentLock' in act)
        return {
            action: {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: {
                    environment: act.createEnvironmentLock.environment,
                    lockId: randomLockId(),
                    message: m,
                },
            },
        };
    if ('createApplicationLock' in act)
        return {
            action: {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: {
                    environment: act.createApplicationLock.environment,
                    application: act.createApplicationLock.application,
                    lockId: randomLockId(),
                    message: m,
                },
            },
        };
    if ('deleteEnvironmentLock' in act)
        return {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: act.deleteEnvironmentLock.environment,
                    lockId: act.deleteEnvironmentLock.lockId,
                },
            },
        };
    if ('deleteApplicationLock' in act)
        return {
            action: {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: {
                    environment: act.deleteApplicationLock.environment,
                    application: act.deleteApplicationLock.application,
                    lockId: act.deleteApplicationLock.lockId,
                },
            },
        };
    if ('deploy' in act)
        return {
            action: {
                $case: 'deploy',
                deploy: {
                    environment: act.deploy.environment,
                    application: act.deploy.application,
                    version: act.deploy.version,
                    lockBehavior: LockBehavior.Ignore,
                    ignoreAllLocks: false,
                },
            },
        };
    if ('prepareUndeploy' in act)
        return {
            action: {
                $case: 'prepareUndeploy',
                prepareUndeploy: {
                    application: act.prepareUndeploy.application,
                },
            },
        };
    if ('undeploy' in act)
        return {
            action: {
                $case: 'undeploy',
                undeploy: {
                    application: act.undeploy.application,
                },
            },
        };
    else return {};
};
