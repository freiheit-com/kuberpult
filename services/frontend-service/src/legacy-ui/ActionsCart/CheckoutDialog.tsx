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
import { useCallback, useContext, useState, VFC } from 'react';
import {
    Alert,
    AlertTitle,
    Button,
    CircularProgress,
    Dialog,
    DialogTitle,
    IconButton,
    Snackbar,
    TextField,
    Typography,
} from '@material-ui/core';
import { Close } from '@material-ui/icons';
import { ActionsCartContext } from '../App';
import { BatchAction, GetOverviewResponse } from '../../api/api';
import { useUnaryCallback } from '../Api';
import { addMessageToAction, CartAction, hasLockAction, isNonNullable } from '../ActionDetails';

export const callbacks = {
    useBatch: (acts: BatchAction[], success?: () => void, fail?: () => void) =>
        useUnaryCallback(
            useCallback(
                (api, authHeader) =>
                    api
                        .batchService()
                        .ProcessBatch(
                            {
                                actions: acts,
                            },
                            authHeader as any
                        )
                        .then(success)
                        .catch(fail),
                [acts, success, fail]
            )
        ),
};

const ApplyButton: VFC<{ doActions: () => void; state: string }> = ({ doActions, state }) => {
    switch (state) {
        case 'rejected':
        case 'resolved':
        case 'waiting':
            return (
                <Button variant="contained" onClick={doActions}>
                    Yes
                </Button>
            );
        case 'pending':
            return (
                <Button variant={'contained'} disabled>
                    <CircularProgress size={20} />
                </Button>
            );
        default:
            return (
                <Button variant={'contained'} disabled>
                    Failed
                </Button>
            );
    }
};

const LockMessageInput: VFC<{ updateMessage: (e: any) => void }> = ({ updateMessage }) => (
    <TextField
        label="Lock Message"
        variant="outlined"
        sx={{ m: 1 }}
        placeholder="default-lock"
        onChange={updateMessage}
        className="actions-cart__lock-message"
    />
);

const CheckoutButton: VFC<{ openDialog: () => void; disabled: boolean }> = ({ openDialog, disabled }) => (
    <Button sx={{ display: 'flex' }} onClick={openDialog} variant={'contained'} disabled={disabled}>
        <Typography variant="h6">
            <strong>Apply</strong>
        </Typography>
    </Button>
);

export const SyncWindowsWarning: VFC<{
    actions: CartAction[];
    overview: GetOverviewResponse;
}> = ({ actions, overview }) => {
    const anyAppInActionsHasSyncWindows = actions
        .map((a) => {
            if ('deploy' in a) {
                const environmentName = a.deploy.environment;
                const applicationName = a.deploy.application;
                const numSyncWindows =
                    overview.environments[environmentName].applications[applicationName]?.argoCD?.syncWindows.length ??
                    0;
                return numSyncWindows > 0;
            } else {
                return false;
            }
        })
        .some((appHasSyncWindows) => appHasSyncWindows);
    if (anyAppInActionsHasSyncWindows) {
        return (
            <Alert variant="outlined" sx={{ m: 1 }} severity="warning">
                <AlertTitle>ArgoCD sync windows are active for at least one application!</AlertTitle>
                <p>Warning: This can delay deployment.</p>
            </Alert>
        );
    } else {
        return null;
    }
};

export const CheckoutCart: VFC<{ overview: GetOverviewResponse }> = ({ overview }) => {
    const [notify, setNotify] = useState({ open: false, message: '' });
    const [dialogOpen, setDialogOpen] = useState(false);
    const { actions, setActions } = useContext(ActionsCartContext);
    const [lockMessage, setLockMessage] = useState('');

    const updateLockMessage = useCallback((e) => setLockMessage(e.target.value), [setLockMessage]);

    const openNotification = useCallback(
        (msg: string) => {
            setNotify({ open: true, message: msg });
        },
        [setNotify]
    );

    const closeNotification = useCallback(
        (event: React.SyntheticEvent | React.MouseEvent, reason?: string) => {
            if (reason === 'clickaway') {
                return;
            }
            setNotify({ open: false, message: '' });
        },
        [setNotify]
    );

    const openDialog = useCallback(() => {
        setDialogOpen(true);
    }, [setDialogOpen]);

    const closeDialog = useCallback(() => {
        setDialogOpen(false);
    }, [setDialogOpen]);

    const onActionsSucceeded = useCallback(() => {
        setActions([]);
        closeDialog();
        setLockMessage('');
        openNotification('Actions were applied successfully!');
    }, [setActions, openNotification, closeDialog, setLockMessage]);
    const onActionsFailed = useCallback(() => {
        closeDialog();
        openNotification('Actions were not applied. Please try again!');
    }, [openNotification, closeDialog]);

    const actionsWithMessage = actions.map((act) => addMessageToAction(act, lockMessage)).filter(isNonNullable);
    const [doActions, doActionsState] = callbacks.useBatch(actionsWithMessage, onActionsSucceeded, onActionsFailed);

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
            }}>
            <SyncWindowsWarning actions={actions} overview={overview} />
            {hasLockAction(actions) && <LockMessageInput updateMessage={updateLockMessage} />}
            <CheckoutButton
                openDialog={openDialog}
                disabled={actions.length === 0 || dialogOpen || (hasLockAction(actions) && lockMessage === '')}
            />
            <Dialog onClose={closeDialog} open={dialogOpen}>
                <DialogTitle sx={{ m: 0, p: 2 }}>
                    <Typography variant="subtitle1" component="div" className="checkout-title">
                        <span>{'Are you sure you want to apply all planned actions?'}</span>
                    </Typography>
                </DialogTitle>
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={closeDialog}>Cancel</Button>
                    <ApplyButton doActions={doActions} state={doActionsState.state} />
                </span>
            </Dialog>
            <Snackbar
                open={notify.open}
                autoHideDuration={6000}
                onClose={closeNotification}
                message={notify.message}
                action={closeIcon}
            />
        </div>
    );
};
