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

type ActionDetails = {
    type: ActionTypes;
    name: string;
    summary: string;
    dialogTitle: string;
    notMessageSuccess: string;
    notMessageFail: string;
    description?: string;
    icon: React.ReactElement;

    // action details optional
    environment?: string;
    application?: string;
    lockId?: string;
    lockMessage?: string;
    version?: number;
};
export const GetActionDetails = (action: BatchAction): ActionDetails => {
    switch (action.action?.$case) {
        case 'deploy':
            return {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Are you sure you want to deploy this version?',
                notMessageSuccess:
                    'Version ' +
                    action.action?.deploy.version +
                    ' was successfully deployed to ' +
                    action.action?.deploy.environment,
                notMessageFail: 'Deployment failed',
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
        case 'createEnvironmentLock':
            return {
                type: ActionTypes.CreateEnvironmentLock,
                name: 'Create Env Lock',
                dialogTitle: 'Are you sure you want to add this environment lock?',
                notMessageSuccess:
                    'New environment lock on ' +
                    action.action?.createEnvironmentLock.environment +
                    ' was successfully created with message: ' +
                    action.action?.createEnvironmentLock.message,
                notMessageFail: 'Creating new environment lock failed',
                summary: 'Create new environment lock on ' + action.action?.createEnvironmentLock.environment,
                icon: <LockRounded />,
                environment: action.action?.createEnvironmentLock.environment,
                lockId: action.action?.createEnvironmentLock.lockId,
                lockMessage: action.action?.createEnvironmentLock.message,
            };
        case 'createEnvironmentApplicationLock':
            return {
                type: ActionTypes.CreateApplicationLock,
                name: 'Create App Lock',
                dialogTitle: 'Are you sure you want to add this application lock?',
                notMessageSuccess:
                    'New application lock on ' +
                    action.action?.createEnvironmentApplicationLock.environment +
                    ' was successfully created with message: ' +
                    action.action?.createEnvironmentApplicationLock.message,
                notMessageFail: 'Creating new application lock failed',
                summary:
                    'Lock "' +
                    action.action?.createEnvironmentApplicationLock.application +
                    '" on ' +
                    action.action?.createEnvironmentApplicationLock.environment,
                icon: <LockRounded />,
                environment: action.action?.createEnvironmentApplicationLock.environment,
                application: action.action?.createEnvironmentApplicationLock.application,
                lockId: action.action?.createEnvironmentApplicationLock.lockId,
                lockMessage: action.action?.createEnvironmentApplicationLock.message,
            };
        case 'deleteEnvironmentLock':
            return {
                type: ActionTypes.DeleteEnvironmentLock,
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                notMessageSuccess:
                    'Environment lock on ' +
                    action.action?.deleteEnvironmentLock.environment +
                    ' was successfully deleted',
                notMessageFail: 'Deleting environment lock failed',
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
                notMessageSuccess:
                    'Application lock on ' +
                    action.action?.deleteEnvironmentApplicationLock.environment +
                    ' was successfully deleted',
                notMessageFail: 'Deleting application lock failed',
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
                notMessageSuccess:
                    'Undeploy version for Application ' +
                    action.action?.prepareUndeploy.application +
                    ' was successfully created',
                notMessageFail: 'Undeploy version failed',
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
                notMessageSuccess:
                    'Application ' + action.action?.undeploy.application + ' was successfully un-deployed',
                notMessageFail: 'Undeploy application failed',
                summary: 'Undeploy and delete Application ' + action.action?.undeploy.application,
                icon: <DeleteForeverRounded />,
                application: action.action?.undeploy.application,
            };
        default:
            return {
                type: ActionTypes.UNKNOWN,
                name: 'invalid',
                dialogTitle: 'invalid',
                notMessageSuccess: 'invalid',
                notMessageFail: 'invalid',
                summary: 'invalid',
                icon: <Error />,
            };
    }
};
