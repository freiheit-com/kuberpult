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
import { BatchAction } from '../api/api';
import {
    DeleteForeverRounded,
    DeleteOutlineRounded,
    Error,
    LockOpenRounded,
    LockRounded,
    MoveToInboxRounded,
} from '@material-ui/icons';
import * as React from 'react';

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

export type CartAction = BatchAction | CreateLockDetails;

export interface CreateLockDetails {
    action?:
        | {
              $case: 'environmentLockDetails';
              environmentLockDetails: {
                  environment: string;
                  lockId: string;
              };
          }
        | {
              $case: 'environmentApplicationLockDetails';
              environmentApplicationLockDetails: {
                  environment: string;
                  application: string;
                  lockId: string;
              };
          };
}

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
    switch (action.action?.$case) {
        case 'deploy':
            return {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Are you sure you want to deploy this version?',
                summary:
                    'Deploy version ' +
                    action.action?.deploy.version +
                    ' of "' +
                    action.action?.deploy.application +
                    '" to ' +
                    action.action?.deploy.environment,
                icon: <MoveToInboxRounded />,
                environment: action.action?.deploy.environment,
                application: action.action?.deploy.application,
                version: action.action?.deploy.version,
            };
        case 'environmentLockDetails':
            return {
                type: ActionTypes.CreateEnvironmentLock,
                name: 'Create Env Lock',
                dialogTitle: 'Are you sure you want to add this environment lock?',
                summary: 'Create new environment lock on ' + action.action?.environmentLockDetails.environment,
                icon: <LockRounded />,
                environment: action.action?.environmentLockDetails.environment,
                lockId: action.action?.environmentLockDetails.lockId,
            };
        case 'environmentApplicationLockDetails':
            return {
                type: ActionTypes.CreateApplicationLock,
                name: 'Create App Lock',
                dialogTitle: 'Are you sure you want to add this application lock?',
                summary:
                    'Lock "' +
                    action.action?.environmentApplicationLockDetails.application +
                    '" on ' +
                    action.action?.environmentApplicationLockDetails.environment,
                icon: <LockRounded />,
                environment: action.action?.environmentApplicationLockDetails.environment,
                application: action.action?.environmentApplicationLockDetails.application,
                lockId: action.action?.environmentApplicationLockDetails.lockId,
            };
        case 'deleteEnvironmentLock':
            return {
                type: ActionTypes.DeleteEnvironmentLock,
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                summary: 'Delete environment lock on ' + action.action?.deleteEnvironmentLock.environment,
                icon: <LockOpenRounded />,
                environment: action.action?.deleteEnvironmentLock.environment,
                lockId: action.action?.deleteEnvironmentLock.lockId,
            };
        case 'deleteEnvironmentApplicationLock':
            return {
                type: ActionTypes.DeleteApplicationLock,
                name: 'Delete App Lock',
                dialogTitle: 'Are you sure you want to delete this application lock?',
                summary:
                    'Unlock "' +
                    action.action?.deleteEnvironmentApplicationLock.application +
                    '" on ' +
                    action.action?.deleteEnvironmentApplicationLock.environment,
                icon: <LockOpenRounded />,
                environment: action.action?.deleteEnvironmentApplicationLock.environment,
                application: action.action?.deleteEnvironmentApplicationLock.application,
                lockId: action.action?.deleteEnvironmentApplicationLock.lockId,
            };
        case 'prepareUndeploy':
            return {
                type: ActionTypes.PrepareUndeploy,
                name: 'Prepare Undeploy',
                dialogTitle: 'Are you sure you want to start undeploy?',
                description:
                    'The new version will go through the same cycle as any other versions' +
                    ' (e.g. development->staging->production). ' +
                    'The behavior is similar to any other version that is created normally.',
                summary: 'Prepare undeploy version for Application ' + action.action?.prepareUndeploy.application,
                icon: <DeleteOutlineRounded />,
                application: action.action?.prepareUndeploy.application,
            };
        case 'undeploy':
            return {
                type: ActionTypes.Undeploy,
                name: 'Undeploy',
                dialogTitle: 'Are you sure you want to undeploy this application?',
                description: 'This application will be deleted permanently',
                summary: 'Undeploy and delete Application ' + action.action?.undeploy.application,
                icon: <DeleteForeverRounded />,
                application: action.action?.undeploy.application,
            };
        default:
            return {
                type: ActionTypes.UNKNOWN,
                name: 'invalid',
                dialogTitle: 'invalid',
                summary: 'invalid',
                icon: <Error />,
            };
    }
};
