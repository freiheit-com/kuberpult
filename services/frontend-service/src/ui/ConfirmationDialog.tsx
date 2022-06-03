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
import { Lock } from '../api/api';
import * as React from 'react';
import { useCallback, useContext } from 'react';
import { Alert, AlertTitle, Button, Dialog, DialogTitle, IconButton, Snackbar, Typography } from '@material-ui/core';
import { Close, LockRounded } from '@material-ui/icons';
import { ActionsCartContext } from './App';
import { ActionTypes, CartAction, getActionDetails, isDeployAction } from './ActionDetails';

const inCart = (actions: CartAction[], action: CartAction) =>
    actions ? actions.find((act) => JSON.stringify(act) === JSON.stringify(action)) : false;

const getCartConflicts = (cartActions: CartAction[], newAction: CartAction) => {
    const conflicts = new Set<CartAction>();
    for (const action of cartActions) {
        const act = getActionDetails(action);
        const newAct = getActionDetails(newAction);

        if (isDeployAction(newAction) && isDeployAction(action)) {
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
    }
    return conflicts;
};

export const exportedForTesting = {
    getCartConflicts: getCartConflicts,
};

export interface ConfirmationDialogProviderProps {
    children: React.ReactElement;
    action: CartAction;
    locks?: [string, Lock][];
    fin?: () => void;
}

export const ConfirmationDialogProvider = (props: ConfirmationDialogProviderProps) => {
    const { action, locks, fin } = props;
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
        if (conflicts.size || locks?.length) {
            setDialogOpen(true);
        } else {
            addAction();
        }
    }, [setDialogOpen, addAction, locks, conflicts]);

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

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

    return (
        <>
            {React.cloneElement(props.children, {
                inCart: inCart(actions, action),
                addToCart: handleAddToCart,
            })}
            <Dialog onClose={closeDialog} open={dialogOpen}>
                <DialogTitle sx={{ m: 0, p: 2 }}>
                    <Typography variant="subtitle1" component="div" className="confirmation-title">
                        <span>{getActionDetails(action).dialogTitle}</span>
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
                <div style={{ margin: '16px 24px' }}>{getActionDetails(action).description}</div>
                {deployLocks}
                {conflictMessage}
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
