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
import { Button } from '../button';
import { DeleteGray, HideBarWhite } from '../../../images';
import { BatchAction } from '../../../api/api';
import {
    deleteAction,
    useActions,
    deleteAllActions,
    useNumberOfActions,
    showSnackbarSuccess,
    showSnackbarError,
    useAllLocks,
    DisplayLock,
    randomLockId,
    addAction,
    useLocksSimilarTo,
    useRelease,
    useLocksConflictingWithActions,
} from '../../utils/store';
import React, { ChangeEvent, useCallback, useMemo, useState } from 'react';
import { useApi } from '../../utils/GrpcApi';
import { TextField } from '@material-ui/core';
import classNames from 'classnames';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { Spinner } from '../Spinner/Spinner';
import { ReleaseVersionWithLinks } from '../ReleaseVersion/ReleaseVersion';
import { DisplayLockInlineRenderer } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';
import { ConfirmationDialog } from '../dialog/ConfirmationDialog';

export enum ActionTypes {
    Deploy,
    PrepareUndeploy,
    Undeploy,
    CreateEnvironmentLock,
    DeleteEnvironmentLock,
    CreateApplicationLock,
    DeleteApplicationLock,
    DeleteEnvFromApp,
    UNKNOWN,
}

export type ActionDetails = {
    type: ActionTypes;
    name: string;
    summary: string;
    dialogTitle: string;
    tooltip: string;

    // action details optional
    environment?: string;
    application?: string;
    lockId?: string;
    lockMessage?: string;
    version?: number;
};

export const getActionDetails = (
    { action }: BatchAction,
    appLocks: DisplayLock[],
    envLocks: DisplayLock[]
): ActionDetails => {
    switch (action?.$case) {
        case 'createEnvironmentLock':
            return {
                type: ActionTypes.CreateEnvironmentLock,
                name: 'Create Env Lock',
                dialogTitle: 'Are you sure you want to add this environment lock?',
                summary: 'Create new environment lock on ' + action.createEnvironmentLock.environment,
                tooltip:
                    'An environment lock will prevent automated process from changing the deployed version - note that kuberpult users can still deploy despite locks.',
                environment: action.createEnvironmentLock.environment,
            };
        case 'deleteEnvironmentLock':
            return {
                type: ActionTypes.DeleteEnvironmentLock,
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                summary:
                    'Delete environment lock on ' +
                    action.deleteEnvironmentLock.environment +
                    ' with the message: "' +
                    envLocks.find((lock) => lock.lockId === action.deleteEnvironmentLock.lockId)?.message +
                    '"',
                tooltip: 'This will only remove the lock, it will not automatically deploy anything.',
                environment: action.deleteEnvironmentLock.environment,
                lockId: action.deleteEnvironmentLock.lockId,
                lockMessage: envLocks.find((lock) => lock.lockId === action.deleteEnvironmentLock.lockId)?.message,
            };
        case 'createEnvironmentApplicationLock':
            return {
                type: ActionTypes.CreateApplicationLock,
                name: 'Create App Lock',
                dialogTitle: 'Are you sure you want to add this application lock?',
                summary:
                    'Create new application lock for "' +
                    action.createEnvironmentApplicationLock.application +
                    '" on ' +
                    action.createEnvironmentApplicationLock.environment,
                tooltip:
                    'An app lock will prevent automated process from changing the deployed version - note that kuberpult users can still deploy despite locks.',
                environment: action.createEnvironmentApplicationLock.environment,
                application: action.createEnvironmentApplicationLock.application,
            };
        case 'deleteEnvironmentApplicationLock':
            return {
                type: ActionTypes.DeleteApplicationLock,
                name: 'Delete App Lock',
                dialogTitle: 'Are you sure you want to delete this application lock?',
                summary:
                    'Delete application lock for "' +
                    action.deleteEnvironmentApplicationLock.application +
                    '" on ' +
                    action.deleteEnvironmentApplicationLock.environment +
                    ' with the message: "' +
                    appLocks.find((lock) => lock.lockId === action.deleteEnvironmentApplicationLock.lockId)?.message +
                    '"',
                tooltip: 'This will only remove the lock, it will not automatically deploy anything.',
                environment: action.deleteEnvironmentApplicationLock.environment,
                application: action.deleteEnvironmentApplicationLock.application,
                lockId: action.deleteEnvironmentApplicationLock.lockId,
                lockMessage: appLocks.find((lock) => lock.lockId === action.deleteEnvironmentApplicationLock.lockId)
                    ?.message,
            };
        case 'deploy':
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
                tooltip: '',
                environment: action.deploy.environment,
                application: action.deploy.application,
                version: action.deploy.version,
            };
        case 'prepareUndeploy':
            return {
                type: ActionTypes.PrepareUndeploy,
                name: 'Prepare Undeploy',
                dialogTitle: 'Are you sure you want to start undeploy?',
                tooltip:
                    'The new version will go through the same cycle as any other versions' +
                    ' (e.g. development->staging->production). ' +
                    'The behavior is similar to any other version that is created normally.',
                summary: 'Prepare undeploy version for Application "' + action.prepareUndeploy.application + '"',
                application: action.prepareUndeploy.application,
            };
        case 'undeploy':
            return {
                type: ActionTypes.Undeploy,
                name: 'Undeploy',
                dialogTitle: 'Are you sure you want to undeploy this application?',
                tooltip: 'This application will be deleted permanently',
                summary: 'Undeploy and delete Application "' + action.undeploy.application + '"',
                application: action.undeploy.application,
            };
        case 'deleteEnvFromApp':
            return {
                type: ActionTypes.DeleteEnvFromApp,
                name: 'Delete an Environment from App',
                dialogTitle: 'Are you sure you want to delete environments from this application?',
                tooltip: 'These environments will be deleted permanently from this application',
                summary:
                    'Delete environment "' +
                    action.deleteEnvFromApp.environment +
                    '" from application "' +
                    action.deleteEnvFromApp.application +
                    '"',
                application: action.deleteEnvFromApp.application,
            };
        default:
            return {
                type: ActionTypes.UNKNOWN,
                name: 'invalid',
                dialogTitle: 'invalid',
                summary: 'invalid',
                tooltip: 'invalid',
            };
    }
};

type SideBarListItemProps = {
    children: BatchAction;
};

export const SideBarListItem: React.FC<{ children: BatchAction }> = ({ children: action }: SideBarListItemProps) => {
    const { environmentLocks, appLocks } = useAllLocks();
    const actionDetails = getActionDetails(action, appLocks, environmentLocks);
    const release = useRelease(actionDetails.application ?? '', actionDetails.version ?? 0);
    const handleDelete = useCallback(() => deleteAction(action), [action]);
    const similarLocks = useLocksSimilarTo(action);
    const handleAddAll = useCallback(() => {
        similarLocks.appLocks.forEach((displayLock: DisplayLock) => {
            if (!displayLock.environment) {
                throw new Error('app lock must have environment set: ' + JSON.stringify(displayLock));
            }
            if (!displayLock.lockId) {
                throw new Error('app lock must have lock id set: ' + JSON.stringify(displayLock));
            }
            if (!displayLock.application) {
                throw new Error('app lock must have application set: ' + JSON.stringify(displayLock));
            }
            const newAction: BatchAction = {
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        environment: displayLock.environment,
                        application: displayLock.application,
                        lockId: displayLock.lockId,
                    },
                },
            };
            addAction(newAction);
        });
        similarLocks.environmentLocks.forEach((displayLock: DisplayLock) => {
            if (!displayLock.environment) {
                throw new Error('env lock must have environment set: ' + JSON.stringify(displayLock));
            }
            if (!displayLock.lockId) {
                throw new Error('env lock must have lock id set: ' + JSON.stringify(displayLock));
            }
            const newAction: BatchAction = {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: displayLock.environment,
                        lockId: displayLock.lockId,
                    },
                },
            };
            addAction(newAction);
        });
    }, [similarLocks]);
    const deleteAllSection =
        similarLocks.environmentLocks.length === 0 && similarLocks.appLocks.length === 0 ? null : (
            <div className="mdc-drawer-sidebar-list-item-delete-all">
                <div
                    title={
                        'Other locks are detected by Lock Id (' +
                        actionDetails.lockId +
                        '). This means these locks were created with the same "Apply" of the planned actions.'
                    }>
                    There are other similar locks.
                </div>
                <Button onClick={handleAddAll} label={' Delete them all! '} className={''}></Button>
            </div>
        );
    return (
        <>
            <div className="mdc-drawer-sidebar-list-item-text" title={actionDetails.tooltip}>
                <div className="mdc-drawer-sidebar-list-item-text-name">{actionDetails.name}</div>
                <div className="mdc-drawer-sidebar-list-item-text-summary">{actionDetails.summary}</div>
                {release !== undefined && actionDetails.application && (
                    <ReleaseVersionWithLinks application={actionDetails.application} release={release} />
                )}
                {deleteAllSection}
            </div>
            <div onClick={handleDelete}>
                <DeleteGray className="mdc-drawer-sidebar-list-item-delete-icon" />
            </div>
        </>
    );
};

export const SideBarList = (): JSX.Element => {
    const actions = useActions();

    return (
        <>
            {actions.map((action, key) => (
                <div key={key} className="mdc-drawer-sidebar-list-item">
                    <SideBarListItem>{action}</SideBarListItem>
                </div>
            ))}
        </>
    );
};

export const SideBar: React.FC<{ className?: string; toggleSidebar: () => void }> = (props) => {
    const { className, toggleSidebar } = props;
    const actions = useActions();
    const [lockMessage, setLockMessage] = useState('');
    const api = useApi;
    const { authHeader, authReady } = useAzureAuthSub((auth) => auth);

    let title = 'Planned Actions';
    const numActions = useNumberOfActions();
    if (numActions > 0) {
        title = 'Planned Actions (' + numActions + ')';
    } else {
        title = 'Planned Actions';
    }
    const lockCreationList = actions.filter(
        (action) =>
            action.action?.$case === 'createEnvironmentLock' ||
            action.action?.$case === 'createEnvironmentApplicationLock'
    );
    const [showSpinner, setShowSpinner] = useState(false);
    const [dialogState, setDialogState] = useState({
        showConfirmationDialog: false,
    });
    const cancelConfirmation = useCallback((): void => {
        setDialogState({ showConfirmationDialog: false });
    }, []);

    const conflictingLocks = useLocksConflictingWithActions();
    const hasLocks = conflictingLocks.environmentLocks.length > 0 || conflictingLocks.appLocks.length > 0;

    const applyActions = useCallback(() => {
        if (lockMessage) {
            lockCreationList.forEach((action) => {
                if (action.action?.$case === 'createEnvironmentLock') {
                    action.action.createEnvironmentLock.message = lockMessage;
                }
                if (action.action?.$case === 'createEnvironmentApplicationLock') {
                    action.action.createEnvironmentApplicationLock.message = lockMessage;
                }
            });
            setLockMessage('');
        }
        if (authReady) {
            setShowSpinner(true);
            const lockId = randomLockId();
            for (const action of actions) {
                if (action.action?.$case === 'createEnvironmentApplicationLock') {
                    action.action.createEnvironmentApplicationLock.lockId = lockId;
                }
                if (action.action?.$case === 'createEnvironmentLock') {
                    action.action.createEnvironmentLock.lockId = lockId;
                }
            }
            api.batchService()
                .ProcessBatch({ actions }, authHeader)
                .then((result) => {
                    deleteAllActions();
                    showSnackbarSuccess('Actions were applied successfully');
                })
                .catch((e) => {
                    // eslint-disable-next-line no-console
                    console.error('error in batch request: ', e);
                    const GrpcErrorPermissionDenied = 7;
                    if (e.code === GrpcErrorPermissionDenied) {
                        showSnackbarError(e.message);
                    } else {
                        showSnackbarError('Actions were not applied. Please try again');
                    }
                })
                .finally(() => {
                    setShowSpinner(false);
                });
            setDialogState({ showConfirmationDialog: false });
        }
    }, [actions, api, authHeader, authReady, lockCreationList, lockMessage]);

    const showDialog = useCallback(() => {
        setDialogState({ showConfirmationDialog: true });
    }, []);

    const newLockExists = useMemo(() => lockCreationList.length !== 0, [lockCreationList.length]);

    const updateMessage = useCallback((e: ChangeEvent<HTMLInputElement>) => {
        setLockMessage(e.target.value);
    }, []);

    const canApply = useMemo(
        () => actions.length > 0 && (!newLockExists || lockMessage),
        [actions.length, lockMessage, newLockExists]
    );
    const appLocksRendered =
        conflictingLocks.appLocks.length === 0 ? undefined : (
            <>
                <h4>Conflicting App Locks:</h4>
                <ul>
                    {conflictingLocks.appLocks.map((appLock: DisplayLock) => (
                        <li>
                            <DisplayLockInlineRenderer
                                lock={appLock}
                                key={appLock.lockId + '-' + appLock.application + '-' + appLock.environment}
                            />
                        </li>
                    ))}
                </ul>
            </>
        );
    const envLocksRendered =
        conflictingLocks.environmentLocks.length === 0 ? undefined : (
            <>
                <h4>Conflicting Environment Locks:</h4>
                <ul>
                    {conflictingLocks.environmentLocks.map((envLock: DisplayLock) => (
                        <li key={envLock.lockId + '-' + envLock.environment + '-envlock'}>
                            <DisplayLockInlineRenderer
                                lock={envLock}
                                key={envLock.lockId + '-' + envLock.environment}
                            />
                        </li>
                    ))}
                </ul>
            </>
        );
    const confirmationDialog: JSX.Element = hasLocks ? (
        <ConfirmationDialog
            headerLabel={'Please Confirm the Deployment over Locks'}
            onConfirm={applyActions}
            confirmLabel={'Confirm Deployment'}
            onCancel={cancelConfirmation}
            open={dialogState.showConfirmationDialog}>
            <div>
                You are attempting to deploy apps, although there are locks present. Please check the locks and be sure
                you really want to ignore them.
                <div className={'locks'}>
                    {envLocksRendered}
                    {appLocksRendered}
                </div>
            </div>
        </ConfirmationDialog>
    ) : (
        <ConfirmationDialog
            headerLabel={'Please Confirm the Planned Actions'}
            onConfirm={applyActions}
            confirmLabel={'Confirm Planned Actions'}
            onCancel={cancelConfirmation}
            open={dialogState.showConfirmationDialog}>
            <div>Are you sure you want to apply all planned actions?</div>
        </ConfirmationDialog>
    );

    return (
        <aside className={className}>
            <nav className="mdc-drawer-sidebar mdc-drawer__drawer sidebar-content">
                <div className="mdc-drawer-sidebar mdc-drawer-sidebar-header">
                    <Button
                        className={'mdc-drawer-sidebar-header__button mdc-button--unelevated'}
                        icon={<HideBarWhite />}
                        onClick={toggleSidebar}
                    />
                    <h1 className="mdc-drawer-sidebar mdc-drawer-sidebar-header-title">{title}</h1>
                </div>
                <nav className="mdc-drawer-sidebar mdc-drawer-sidebar-content">
                    <div className="mdc-drawer-sidebar mdc-drawer-sidebar-list">
                        <SideBarList />
                    </div>
                </nav>
                {newLockExists && (
                    <TextField
                        label="Lock Message"
                        variant="outlined"
                        placeholder="default-lock"
                        onChange={updateMessage}
                        className="actions-cart__lock-message"
                        value={lockMessage}
                    />
                )}
                <div className="mdc-drawer-sidebar mdc-sidebar-sidebar-footer">
                    <Button
                        className={classNames(
                            'mdc-sidebar-sidebar-footer',
                            'mdc-button--unelevated',
                            'mdc-drawer-sidebar-apply-button'
                        )}
                        label={'Apply'}
                        disabled={!canApply}
                        onClick={showDialog}
                    />
                    {showSpinner && <Spinner message="Submitting changes" />}
                    {confirmationDialog}
                </div>
            </nav>
        </aside>
    );
};
