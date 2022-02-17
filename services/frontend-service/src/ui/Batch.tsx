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
import { useUnaryCallback } from './Api';
import * as React from 'react';
import { Button, Dialog, DialogTitle, IconButton, Typography, Snackbar } from '@material-ui/core';
import { useCallback, useContext } from 'react';
import {
    Close,
    DeleteForeverRounded,
    DeleteOutlineRounded,
    LockOpenRounded,
    LockRounded,
    MoveToInboxRounded,
} from '@material-ui/icons';
import { ActionsCartContext } from './App';

export const callbacks = {
    useBatch: (acts: BatchAction[], success?: () => void, fail?: () => void) =>
        useUnaryCallback(
            React.useCallback(
                (api) =>
                    api
                        .batchService()
                        .ProcessBatch({
                            actions: acts,
                        })
                        .then(success)
                        .catch(fail),
                [acts, success, fail]
            )
        ),
};

export interface ConfirmationDialogProviderProps {
    children: React.ReactElement;
    action: BatchAction;
    fin?: () => void;
}

const InCart = (actions: BatchAction[], action: BatchAction) =>
    actions ? actions.find((act) => JSON.stringify(act.action) === JSON.stringify(action.action)) : false;

export const ConfirmationDialogProvider = (props: ConfirmationDialogProviderProps) => {
    const { action, fin } = props;
    const [openNotify, setOpenNotify] = React.useState(false);
    const [openDialog, setOpenDialog] = React.useState(false);
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

    const handleOpen = useCallback(() => {
        setOpenDialog(true);
        setOpenNotify(false);
    }, [setOpenDialog, setOpenNotify]);

    const handleClose = useCallback(() => {
        setOpenDialog(false);
    }, [setOpenDialog]);

    const closeWhenDone = useCallback(() => {
        openNotification();
        if (fin) fin();
        handleClose();
        if (!InCart(actions, action)) {
            setActions([...actions, action]);
        }
    }, [fin, handleClose, openNotification, action, actions, setActions]);

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

    return (
        <>
            {React.cloneElement(props.children, {
                state: InCart(actions, action) ? 'in-cart' : 'not-in-cart',
                openDialog: handleOpen,
            })}
            <Dialog onClose={handleClose} open={openDialog}>
                <DialogTitle>
                    <Typography variant="subtitle1" component="div" className="confirmation-title">
                        <span>{GetActionDetails(action).dialogTitle}</span>
                        <IconButton aria-label="close" color="inherit" onClick={handleClose}>
                            <Close fontSize="small" />
                        </IconButton>
                    </Typography>
                </DialogTitle>
                <div style={{ margin: '16px 24px' }}>{GetActionDetails(action).description}</div>
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={handleClose}>Cancel</Button>
                    <Button onClick={closeWhenDone}>Add to cart</Button>
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
    name: string;
    summary: string;
    dialogTitle: string;
    notMessageSuccess: string;
    notMessageFail: string;
    description?: string;
    icon?: React.ReactElement;
};

export const GetActionDetails = (action: BatchAction): ActionDetails => {
    switch (action.action?.$case) {
        case 'deploy':
            return {
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
            };
        case 'createEnvironmentLock':
            return {
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
            };
        case 'createEnvironmentApplicationLock':
            return {
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
            };
        case 'deleteEnvironmentLock':
            return {
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                notMessageSuccess:
                    'Environment lock on ' +
                    action.action?.deleteEnvironmentLock.environment +
                    ' was successfully deleted',
                notMessageFail: 'Deleting environment lock failed',
                summary: 'Delete environment lock on ' + action.action?.deleteEnvironmentLock.environment,
                icon: <LockOpenRounded />,
            };
        case 'deleteEnvironmentApplicationLock':
            return {
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
            };
        case 'prepareUndeploy':
            return {
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
            };
        case 'undeploy':
            return {
                name: 'Undeploy',
                dialogTitle: 'Are you sure you want to undeploy this application?',
                description: 'This application will be deleted permanently',
                notMessageSuccess:
                    'Application ' + action.action?.undeploy.application + ' was successfully un-deployed',
                notMessageFail: 'Undeploy application failed',
                summary: 'Undeploy and delete Application ' + action.action?.undeploy.application,
                icon: <DeleteForeverRounded />,
            };
        default:
            return {
                name: 'invalid',
                dialogTitle: 'invalid',
                notMessageSuccess: 'invalid',
                notMessageFail: 'invalid',
                summary: 'invalid',
            };
    }
};
