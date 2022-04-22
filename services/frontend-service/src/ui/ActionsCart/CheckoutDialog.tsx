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

const ApplyButton = (props: {
    openNotification: (msg: string) => void;
    closeDialog: () => void;
    lockMessage: string;
}) => {
    const { openNotification, closeDialog, lockMessage } = props;
    const { actions, setActions } = useContext(ActionsCartContext);

    const actionsSucceeded = useCallback(() => {
        setActions([]);
        closeDialog();
        openNotification('Actions were applied successfully!');
    }, [setActions, openNotification, closeDialog]);
    const actionsFailed = useCallback(() => {
        closeDialog();
        openNotification('Actions were not applied. Please try again!');
    }, [openNotification, closeDialog]);

    const newActions = [...actions];
    for (const act of newActions) {
        switch (act.action?.$case) {
            case 'createEnvironmentLock':
                act.action.createEnvironmentLock.message = lockMessage;
                break;
            case 'createEnvironmentApplicationLock':
                act.action.createEnvironmentApplicationLock.message = lockMessage;
                break;
            default:
        }
    }
    setActions(newActions);
    const [doActions, doActionsState] = callbacks.useBatch(newActions, actionsSucceeded, actionsFailed);

    switch (doActionsState.state) {
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

const LockMessageBox = (props: { setMessage: (m: string) => void }) => {
    const { actions, setActions } = useContext(ActionsCartContext);
    const updateMessage = React.useCallback(
        (e) => {
            /*const newActions = actions;
            for (const act of newActions) {
                switch (act.action?.$case) {
                    case 'createEnvironmentLock':
                        act.action.createEnvironmentLock.message = e.target.value;
                        break;
                    case 'createEnvironmentApplicationLock':
                        act.action.createEnvironmentApplicationLock.message = e.target.value;
                        break;
                    default:
                }
            }
            setActions(newActions);*/
            props.setMessage(e.target.value);
        },
        [actions, setActions]
    );
    let createLocks = false;
    for (const act of actions) {
        const t = GetActionDetails(act).type;
        if (t === ActionTypes.CreateEnvironmentLock || t === ActionTypes.CreateApplicationLock) {
            createLocks = true;
        }
    }

    if (!createLocks) {
        return null;
    }
    return (
        <TextField
            label="Lock Message"
            variant="outlined"
            sx={{ m: 1 }}
            placeholder="default-lock"
            onChange={updateMessage}
        />
    );
};
export const CheckoutCart = () => {
    const [openNotify, setOpenNotify] = React.useState(false);
    const [dialogOpen, setDialogOpen] = React.useState(false);
    const [notifyMessage, setNotifyMessage] = React.useState('');
    const { actions } = useContext(ActionsCartContext);
    const [lockMessage, setLockMessage] = useState('default-lock');

    const updateLockMessage = useCallback(
        (m: string) => {
            setLockMessage(m);
        },
        [setLockMessage]
    );

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

    const checkoutButton = (
        <Button
            sx={{ display: 'flex' }}
            onClick={openDialog}
            variant={'contained'}
            disabled={actions.length === 0 || dialogOpen}>
            <Typography variant="h6">
                <strong>Apply</strong>
            </Typography>
        </Button>
    );

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
            <LockMessageBox setMessage={updateLockMessage} />
            {checkoutButton}
            <Dialog onClose={closeDialog} open={dialogOpen}>
                <DialogTitle sx={{ m: 0, p: 2 }}>
                    <Typography variant="subtitle1" component="div" className="checkout-title">
                        <span>{'Are you sure you want to apply all planned actions?'}</span>
                    </Typography>
                </DialogTitle>
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={closeDialog}>Cancel</Button>
                    <ApplyButton
                        openNotification={openNotification}
                        closeDialog={closeDialog}
                        lockMessage={lockMessage}
                    />
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
