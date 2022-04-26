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
import { useCallback, useContext, useState } from 'react';
import {
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
import { BatchAction } from '../../api/api';
import { useUnaryCallback } from '../Api';
import { ActionTypes, GetActionDetails } from '../ConfirmationDialog';

export const callbacks = {
    useBatch: (acts: BatchAction[], success?: () => void, fail?: () => void) =>
        useUnaryCallback(
            useCallback(
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

const ApplyButton = (props: { doActions: () => void; state: string }) => {
    const { doActions, state } = props;
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

const showTextField = (actions: BatchAction[]) => {
    for (const act of actions) {
        const t = GetActionDetails(act).type;
        if (t === ActionTypes.CreateEnvironmentLock || t === ActionTypes.CreateApplicationLock) {
            return true;
        }
    }
    return false;
};

const updateMessage = (act: BatchAction, m: string): BatchAction => {
    switch (act.action?.$case) {
        case 'createEnvironmentLock':
            return {
                action: {
                    ...act.action,
                    createEnvironmentLock: {
                        ...act.action.createEnvironmentLock,
                        message: m,
                    },
                },
            };
        case 'createEnvironmentApplicationLock':
            return {
                action: {
                    ...act.action,
                    createEnvironmentApplicationLock: {
                        ...act.action.createEnvironmentApplicationLock,
                        message: m,
                    },
                },
            };
        default:
            return act;
    }
};

const CheckoutButton = (props: { openDialog: () => void; disabled: boolean; setLockMessage: (m: string) => void }) => {
    const { openDialog, disabled, setLockMessage } = props;
    const { actions } = useContext(ActionsCartContext);
    const updateInput = useCallback((e) => setLockMessage(e.target.value), [setLockMessage]);

    return (
        <>
            {showTextField(actions) && (
                <TextField
                    label="Lock Message"
                    variant="outlined"
                    sx={{ m: 1 }}
                    placeholder="default-lock"
                    onChange={updateInput}
                    className="actions-cart__lock-message"
                />
            )}
            <Button sx={{ display: 'flex' }} onClick={openDialog} variant={'contained'} disabled={disabled}>
                <Typography variant="h6">
                    <strong>Apply</strong>
                </Typography>
            </Button>
        </>
    );
};

export const CheckoutCart = () => {
    const [openNotify, setOpenNotify] = useState(false);
    const [dialogOpen, setDialogOpen] = useState(false);
    const [notifyMessage, setNotifyMessage] = useState('');
    const { actions, setActions } = useContext(ActionsCartContext);
    const [lockMessage, setLockMessage] = useState('');

    const openNotification = useCallback(
        (msg: string) => {
            setNotifyMessage(msg);
            setOpenNotify(true);
        },
        [setOpenNotify, setNotifyMessage]
    );

    const closeNotification = useCallback(
        (event: React.SyntheticEvent | React.MouseEvent, reason?: string) => {
            if (reason === 'clickaway') {
                return;
            }
            setOpenNotify(false);
        },
        [setOpenNotify]
    );

    const openDialog = useCallback(() => {
        setDialogOpen(true);
    }, [setDialogOpen]);

    const closeDialog = useCallback(() => {
        setDialogOpen(false);
    }, [setDialogOpen]);

    const actionsSucceeded = useCallback(() => {
        setActions([]);
        closeDialog();
        setLockMessage('');
        openNotification('Actions were applied successfully!');
    }, [setActions, openNotification, closeDialog, setLockMessage]);
    const actionsFailed = useCallback(() => {
        closeDialog();
        openNotification('Actions were not applied. Please try again!');
    }, [openNotification, closeDialog]);

    const newActions = actions.map((act) => updateMessage(act, lockMessage));

    const [doActions, doActionsState] = callbacks.useBatch(newActions, actionsSucceeded, actionsFailed);

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
            <CheckoutButton
                openDialog={openDialog}
                disabled={actions.length === 0 || dialogOpen || (showTextField(actions) && lockMessage === '')}
                setLockMessage={setLockMessage}
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
                open={openNotify}
                autoHideDuration={6000}
                onClose={closeNotification}
                message={notifyMessage}
                action={closeIcon}
            />
        </div>
    );
};
