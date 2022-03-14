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
import { Button, Dialog, DialogTitle, IconButton, Typography, Snackbar, CircularProgress } from '@material-ui/core';
import { useCallback, useContext } from 'react';
import { Close } from '@material-ui/icons';
import { ActionsCartContext } from '../App';
import { BatchAction } from '../../api/api';
import { useUnaryCallback } from '../Api';

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

const ApplyButton = (props: { openNotification: (msg: string) => void; closeDialog: () => void }) => {
    const { openNotification, closeDialog } = props;
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
    const [doActions, doActionsState] = callbacks.useBatch(actions, actionsSucceeded, actionsFailed);

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

export const CheckoutCart = () => {
    const [openNotify, setOpenNotify] = React.useState(false);
    const [dialogOpen, setDialogOpen] = React.useState(false);
    const [notifyMessage, setNotifyMessage] = React.useState('');
    const { actions } = useContext(ActionsCartContext);

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
            sx={{ display: 'flex', height: '5%' }}
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
        <>
            {checkoutButton}
            <Dialog onClose={closeDialog} open={dialogOpen}>
                <DialogTitle sx={{ m: 0, p: 2 }}>
                    <Typography variant="subtitle1" component="div" className="checkout-title">
                        <span>{'Are you sure you want to apply all planned actions?'}</span>
                    </Typography>
                </DialogTitle>
                <span style={{ alignSelf: 'end' }}>
                    <Button onClick={closeDialog}>Cancel</Button>
                    <ApplyButton openNotification={openNotification} closeDialog={closeDialog} />
                </span>
            </Dialog>
            <Snackbar
                open={openNotify}
                autoHideDuration={6000}
                onClose={closeNotification}
                message={notifyMessage}
                action={closeIcon}
            />
        </>
    );
};
