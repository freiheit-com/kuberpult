import DeleteForeverRounded from '@material-ui/icons/DeleteForeverRounded';
import DeleteOutlineRounded from '@material-ui/icons/DeleteOutlineRounded';
import Error from '@material-ui/icons/Error';
import LockOpenRounded from '@material-ui/icons/LockOpenRounded';
import LockRounded from '@material-ui/icons/LockRounded';
import MoveToInboxRounded from '@material-ui/icons/MoveToInboxRounded';
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

// CartAction is the type that is used in the front end and constitutes the Planned Actions.
// when a plan is being applied, these cart actions will be transformed into BatchActions which is the type used by the api
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

const deployActions = [ActionTypes.Deploy, ActionTypes.PrepareUndeploy, ActionTypes.Undeploy] as const;
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
            dialogTitle: 'Please be aware:',
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
    else if ('createEnvironmentLock' in action)
        return {
            type: ActionTypes.CreateEnvironmentLock,
            name: 'Create Env Lock',
            dialogTitle: 'Are you sure you want to add this environment lock?',
            summary: 'Create new environment lock on ' + action.createEnvironmentLock.environment,
            icon: <LockRounded />,
            environment: action.createEnvironmentLock.environment,
        };
    else if ('createApplicationLock' in action)
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
    else if ('deleteEnvironmentLock' in action)
        return {
            type: ActionTypes.DeleteEnvironmentLock,
            name: 'Delete Env Lock',
            dialogTitle: 'Are you sure you want to delete this environment lock?',
            summary: 'Delete environment lock on ' + action.deleteEnvironmentLock.environment,
            icon: <LockOpenRounded />,
            environment: action.deleteEnvironmentLock.environment,
            lockId: action.deleteEnvironmentLock.lockId,
        };
    else if ('deleteApplicationLock' in action)
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
    else if ('prepareUndeploy' in action)
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
    else if ('undeploy' in action)
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

// randBase36 Generates a random id that matches with [0-9A-Z]{7}
// https://en.wikipedia.org/wiki/Base36
const randBase36 = () => Math.random().toString(36).substring(7);
const randomLockId = () => 'ui-' + randBase36();

export function isNonNullable<T>(value: T): value is NonNullable<T> {
    return value !== undefined && value !== null;
}

export const addMessageToAction = (act: CartAction, m: string): BatchAction | null => {
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
    else if ('createApplicationLock' in act)
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
    else return transformToBatch(act);
};

export const transformToBatch = (act: CartAction): BatchAction | null => {
    if ('createEnvironmentLock' in act)
        return {
            action: {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: {
                    environment: act.createEnvironmentLock.environment,
                    lockId: randomLockId(),
                    message: 'no message provided',
                },
            },
        };
    else if ('createApplicationLock' in act)
        return {
            action: {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: {
                    environment: act.createApplicationLock.environment,
                    application: act.createApplicationLock.application,
                    lockId: randomLockId(),
                    message: 'no message provided',
                },
            },
        };
    else if ('deleteEnvironmentLock' in act)
        return {
            action: {
                $case: 'deleteEnvironmentLock',
                deleteEnvironmentLock: {
                    environment: act.deleteEnvironmentLock.environment,
                    lockId: act.deleteEnvironmentLock.lockId,
                },
            },
        };
    else if ('deleteApplicationLock' in act)
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
    else if ('deploy' in act)
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
    else if ('prepareUndeploy' in act)
        return {
            action: {
                $case: 'prepareUndeploy',
                prepareUndeploy: {
                    application: act.prepareUndeploy.application,
                },
            },
        };
    else if ('undeploy' in act)
        return {
            action: {
                $case: 'undeploy',
                undeploy: {
                    application: act.undeploy.application,
                },
            },
        };
    else return null;
};
