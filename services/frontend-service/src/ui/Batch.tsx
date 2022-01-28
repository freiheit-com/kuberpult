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
import { useCallback } from 'react';
import { Close } from '@material-ui/icons';

export const callbacks = {
    useBatch: (act: BatchAction, fin?: () => void) =>
        useUnaryCallback(
            React.useCallback(
                (api) =>
                    api
                        .batchService()
                        .ProcessBatch({
                            actions: [act],
                        })
                        .finally(fin),
                [act, fin]
            )
        ),
};

export interface ConfirmationDialogProviderProps {
    children: React.ReactElement;
    action: BatchAction;
    fin?: () => void;
}

export const ConfirmationDialogProvider = (props: ConfirmationDialogProviderProps) => {
    const { action, fin } = props;
    const [openNotify, setOpenNotify] = React.useState(false);
    const [openDialog, setOpenDialog] = React.useState(false);

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
    }, [fin, handleClose, openNotification]);

    const [doAction, doActionState] = callbacks.useBatch(action, closeWhenDone);

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

    const actionMessages = getMessages(action);

    return (
        <>
            {React.cloneElement(props.children, { state: doActionState.state, openDialog: handleOpen })}
            <Dialog onClose={handleClose} open={openDialog}>
                <DialogTitle>
                    <Typography variant="subtitle1" component="div" className="confirmation-title">
                        <span>{actionMessages.title}</span>
                        <IconButton aria-label="close" color="inherit" onClick={handleClose}>
                            <Close fontSize="small" />
                        </IconButton>
                    </Typography>
                </DialogTitle>
                <div style={{ margin: '16px 24px' }}>{actionMessages.description}</div>
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={handleClose}>Cancel</Button>
                    <Button onClick={doAction}>Yes</Button>
                </span>
            </Dialog>
            <Snackbar
                open={openNotify}
                autoHideDuration={6000}
                onClose={closeNotification}
                message={
                    doActionState.state === 'resolved'
                        ? actionMessages.notMessageSuccess
                        : actionMessages.notMessageFail
                }
                action={closeIcon}
            />
        </>
    );
};

type BatchMessage = {
    title: string;
    notMessageSuccess: string;
    notMessageFail: string;
    description?: string;
};

const getMessages = (action: BatchAction): BatchMessage => {
    switch (action.action?.$case) {
        case 'deploy':
            return {
                title: 'Are you sure you want to deploy this version?',
                notMessageSuccess:
                    'Version ' +
                    action.action?.deploy.version +
                    ' was successfully deployed to ' +
                    action.action?.deploy.environment,
                notMessageFail: 'Deployment failed',
            };
        case 'createEnvironmentLock':
            return {
                title: 'Are you sure you want to add this environment lock?',
                notMessageSuccess:
                    'New environment lock on ' +
                    action.action?.createEnvironmentLock.environment +
                    ' was successfully created with message: ' +
                    action.action?.createEnvironmentLock.message,
                notMessageFail: 'Creating new environment lock failed',
            };
        case 'createEnvironmentApplicationLock':
            return {
                title: 'Are you sure you want to add this application lock?',
                notMessageSuccess:
                    'New application lock on ' +
                    action.action?.createEnvironmentApplicationLock.environment +
                    ' was successfully created with message: ' +
                    action.action?.createEnvironmentApplicationLock.message,
                notMessageFail: 'Creating new application lock failed',
            };
        case 'deleteEnvironmentLock':
            return {
                title: 'Are you sure you want to delete this environment lock?',
                notMessageSuccess:
                    'Environment lock on ' +
                    action.action?.deleteEnvironmentLock.environment +
                    ' was successfully deleted',
                notMessageFail: 'Deleting environment lock failed',
            };
        case 'deleteEnvironmentApplicationLock':
            return {
                title: 'Are you sure you want to delete this application lock?',
                notMessageSuccess:
                    'Application lock on ' +
                    action.action?.deleteEnvironmentApplicationLock.environment +
                    ' was successfully deleted',
                notMessageFail: 'Deleting application lock failed',
            };
        case 'prepareUndeploy':
            return {
                title: 'Are you sure you want to start undeploy?',
                description:
                    'The new version will go through the same cycle as any other versions' +
                    ' (e.g. development->staging->production). ' +
                    'The behavior is similar to any other version that is created normally.',
                notMessageSuccess:
                    'Undeploy version for Application ' +
                    action.action?.prepareUndeploy.application +
                    ' was successfully created',
                notMessageFail: 'Undeploy version failed',
            };
        case 'undeploy':
            return {
                title: 'Are you sure you want to undeploy this application?',
                description: 'This application will be deleted permanently',
                notMessageSuccess:
                    'Application ' + action.action?.undeploy.application + ' was successfully un-deployed',
                notMessageFail: 'Undeploy application failed',
            };
        default:
            return {
                title: 'invalid',
                notMessageSuccess: 'invalid',
                notMessageFail: 'invalid',
            };
    }
};
