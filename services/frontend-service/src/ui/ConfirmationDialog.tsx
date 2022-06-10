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
import { BatchAction, Environment_Application_SyncWindow, Lock } from '../api/api';
import * as React from 'react';
import { Button, Dialog, DialogTitle, IconButton, Typography, Snackbar, Alert, AlertTitle } from '@material-ui/core';
import { useCallback, useContext } from 'react';
import {
    Close,
    DeleteForeverRounded,
    DeleteOutlineRounded,
    Error,
    LockOpenRounded,
    LockRounded,
    MoveToInboxRounded,
} from '@material-ui/icons';
import { ActionsCartContext } from './App';
import { SyncWindow } from './ReleaseDialog';

enum ActionTypes {
    Deploy,
    PrepareUndeploy,
    Undeploy,
    CreateEnvironmentLock,
    DeleteEnvironmentLock,
    CreateApplicationLock,
    DeleteApplicationLock,
    UNKNOWN,
}

const inCart = (actions: BatchAction[], action: BatchAction) =>
    actions ? actions.find((act) => JSON.stringify(act.action) === JSON.stringify(action.action)) : false;

const isDeployment = (t: ActionTypes) =>
    t === ActionTypes.Deploy || t === ActionTypes.PrepareUndeploy || t === ActionTypes.Undeploy;

const getCartConflicts = (cartActions: BatchAction[], newAction: BatchAction) => {
    const conflicts = new Set<BatchAction>();
    for (const action of cartActions) {
        const act = GetActionDetails(action);
        const newAct = GetActionDetails(newAction);

        if (isDeployment(newAct.type) && isDeployment(act.type)) {
            if (newAct.application === act.application) {
                // same app
                if (newAct.type === ActionTypes.Deploy && newAct.type === act.type) {
                    // both are deploy actions check env
                    if (newAct.environment === act.environment) {
                        // conflict, version doesn't matter
                        conflicts.add(action);
                    }
                } else {
                    // either one or both are Un-deploying the same app
                    conflicts.add(action);
                }
            }
        }

        if (newAct.type === ActionTypes.CreateEnvironmentLock && act.type === newAct.type) {
            if (newAct.environment === act.environment) {
                // conflict, locking the same env twice
                conflicts.add(action);
            }
        }

        if (newAct.type === ActionTypes.CreateApplicationLock && act.type === newAct.type) {
            if (newAct.environment === act.environment && newAct.application === act.application) {
                // conflict, locking the same app/env twice
                conflicts.add(action);
            }
        }
    }
    return conflicts;
};

export const exportedForTesting = {
    getCartConflicts: getCartConflicts,
};

export interface ConfirmationDialogProviderProps {
    children: React.ReactElement;
    action: BatchAction;
    locks?: [string, Lock][];
    undeployedUpstream?: string;
    fin?: () => void;
    syncWindows?: Environment_Application_SyncWindow[];
}

export const ConfirmationDialogProvider = (props: ConfirmationDialogProviderProps) => {
    const { action, locks, fin, undeployedUpstream, syncWindows } = props;
    const [openNotify, setOpenNotify] = React.useState(false);
    const [dialogOpen, setDialogOpen] = React.useState(false);
    const { actions, setActions } = useContext(ActionsCartContext);

    const openNotification = useCallback(() => {
        setOpenNotify(true);
    }, [setOpenNotify]);

    const closeNotification = useCallback(
        (event: React.SyntheticEvent | React.MouseEvent, reason?: string) => {
            if (reason === 'clickaway') {
                return;
            }
            setOpenNotify(false);
        },
        [setOpenNotify]
    );

    const closeDialog = useCallback(() => {
        setDialogOpen(false);
    }, [setDialogOpen]);

    const addAction = useCallback(() => {
        openNotification();
        if (fin) fin();
        closeDialog();
        setActions([...actions, action]);
    }, [fin, closeDialog, openNotification, action, actions, setActions]);

    const conflicts = getCartConflicts(actions, action);

    const replaceAction = useCallback(() => {
        openNotification();
        if (fin) fin();
        closeDialog();
        setActions([...actions.filter((v) => !conflicts.has(v)), action]);
    }, [fin, closeDialog, openNotification, action, actions, setActions, conflicts]);

    const handleAddToCart = useCallback(() => {
        if (conflicts.size || locks?.length || undeployedUpstream) {
            setDialogOpen(true);
        } else {
            addAction();
        }
    }, [setDialogOpen, addAction, locks, conflicts, undeployedUpstream]);

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

    const undeployedUpstreamMessage = undeployedUpstream ? (
        <Alert variant="outlined" sx={{ m: 1 }} severity="info">
            <AlertTitle>Warning: Not deployed to "{undeployedUpstream}" yet!</AlertTitle>
            {[
                `This version is not yet deployed to "${undeployedUpstream}" environment.`,
                'Your changes may be overridden by the next release train.',
                `We suggest to first deploy this version to the "${undeployedUpstream}" environment.`,
            ].map((line, id) => (
                <div style={{ display: 'flex', alignItems: 'center' }} key={id}>
                    <strong>{line}</strong>
                </div>
            ))}
        </Alert>
    ) : null;

    const deployLocks = locks?.length ? (
        <Alert variant="outlined" sx={{ m: 1 }} severity="info">
            <AlertTitle>Warning: this application or environment is currently locked!</AlertTitle>
            {locks?.map((lock) => (
                <div style={{ display: 'flex', alignItems: 'center' }} key={lock[0]}>
                    <LockRounded />
                    <strong>{'Lock ID: ' + lock[0] + ' | Message: ' + lock[1].message}</strong>
                </div>
            ))}
        </Alert>
    ) : null;
    const conflictMessage = conflicts.size > 0 && (
        <Alert variant="outlined" sx={{ m: 1 }} severity="error">
            <strong>Possible conflict with actions already in cart!</strong>
        </Alert>
    );
    const syncWindowsMessage =
        syncWindows && syncWindows.length > 0 ? (
            <Alert variant="outlined" sx={{ m: 1 }} severity="warning">
                <AlertTitle>ArgoCD sync windows are active for this application!</AlertTitle>
                <p>Warning: This can delay deployment.</p>
                <h3>Sync windows:</h3>
                <ul>
                    {syncWindows?.map((w, n) => (
                        <li key={`${n}:${w}`}>
                            <SyncWindow w={w} />
                        </li>
                    ))}
                </ul>
            </Alert>
        ) : null;

    return (
        <>
            {React.cloneElement(props.children, {
                inCart: inCart(actions, action),
                addToCart: handleAddToCart,
            })}
            <Dialog onClose={closeDialog} open={dialogOpen}>
                <DialogTitle sx={{ m: 0, p: 2 }}>
                    <Typography variant="subtitle1" component="div" className="confirmation-title">
                        <span>{GetActionDetails(action).dialogTitle}</span>
                    </Typography>
                    <IconButton
                        sx={{
                            position: 'absolute',
                            right: 8,
                            top: 8,
                            color: (theme) => theme.palette.grey[500],
                        }}
                        aria-label="close"
                        color="inherit"
                        onClick={closeDialog}>
                        <Close fontSize="small" />
                    </IconButton>
                </DialogTitle>
                <div style={{ margin: '16px 24px' }}>{GetActionDetails(action).description}</div>
                {deployLocks}
                {undeployedUpstreamMessage}
                {conflictMessage}
                {syncWindowsMessage}
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={closeDialog}>Cancel</Button>
                    <Button onClick={addAction}>Add anyway</Button>
                    {conflicts.size > 0 && <Button onClick={replaceAction}>Replace</Button>}
                </span>
            </Dialog>
            <Snackbar
                open={openNotify}
                autoHideDuration={6000}
                onClose={closeNotification}
                message={'Action added to cart successfully!'}
                action={closeIcon}
            />
        </>
    );
};

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
                dialogTitle: 'Please be aware:',
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
                summary:
                    'Create new environment lock on ' +
                    action.action?.createEnvironmentLock.environment +
                    '. | Lock Message: ' +
                    action.action?.createEnvironmentLock.message,
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
                    action.action?.createEnvironmentApplicationLock.environment +
                    '. | Lock Message: ' +
                    action.action?.createEnvironmentApplicationLock.message,
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
