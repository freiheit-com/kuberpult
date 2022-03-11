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
import { useCallback, useContext } from 'react';
import { useBeforeunload } from 'react-beforeunload';
import {
    Avatar,
    Button,
    CircularProgress,
    Drawer,
    IconButton,
    List,
    ListItem,
    ListItemAvatar,
    ListItemText,
    Paper,
    Snackbar,
    Typography,
} from '@material-ui/core';

import { ClearRounded, Close } from '@material-ui/icons';
import { ActionsCartContext } from '../App';
import { BatchAction } from '../../api/api';
import { GetActionDetails } from '../ConfirmationDialog';
import { theme } from '../App/styles';
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

const ActionListItem = (props: { act: BatchAction; index: number }) => {
    const { act, index } = props;
    const { actions, setActions } = useContext(ActionsCartContext);
    const removeItem = useCallback(() => {
        setActions(actions.filter((_, i) => i !== index));
    }, [actions, setActions, index]);

    return (
        <ListItem divider={true}>
            <ListItemAvatar>
                <Avatar>{GetActionDetails(act).icon}</Avatar>
            </ListItemAvatar>
            <ListItemText primary={GetActionDetails(act).name} secondary={GetActionDetails(act).summary} />
            <IconButton onClick={removeItem}>
                <ClearRounded />
            </IconButton>
        </ListItem>
    );
};

const ActionsList = (props: { openNotification: (msg: string) => void }) => {
    const { openNotification } = props;
    const { actions, setActions } = useContext(ActionsCartContext);
    const actionsSucceeded = useCallback(() => {
        setActions([]);
        openNotification('Actions were applied successfully!');
    }, [setActions, openNotification]);
    const actionsFailed = useCallback(() => {
        openNotification('Actions were not applied. Please try again!');
    }, [openNotification]);
    const [doActions, doActionsState] = callbacks.useBatch(actions, actionsSucceeded, actionsFailed);

    useBeforeunload((e) => {
        if (actions.length) {
            e.preventDefault();
        }
    });

    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                height: '100vh',
            }}>
            <List className="actions" sx={{ width: '100%', bgcolor: 'background.paper' }}>
                {actions.map((act: BatchAction, index: number) => (
                    <ActionListItem act={act} index={index} key={index} />
                ))}
            </List>
            {actions.length === 0 ? (
                <Typography variant="h6" whiteSpace={'pre-line'} align={'center'}>
                    {'Cart Is Currently Empty,\nPlease Add Actions!'}
                </Typography>
            ) : null}
            <ApplyButton actions={actions} doActions={doActions} state={doActionsState.state} />
        </div>
    );
};

const ApplyButton = (props: { actions: BatchAction[]; doActions: () => void; state: string }) => {
    const { actions, doActions, state } = props;
    if (actions.length === 0) {
        return (
            <Button sx={{ display: 'flex', height: '5%' }} variant={'contained'} disabled>
                <Typography variant="h6">
                    <strong>Apply</strong>
                </Typography>
            </Button>
        );
    } else {
        switch (state) {
            case 'rejected':
            case 'resolved':
            case 'waiting':
                return (
                    <Button
                        sx={{ display: 'flex', height: '5%' }}
                        onClick={doActions}
                        variant={'contained'}
                        disabled={actions.length === 0}>
                        <Typography variant="h6">
                            <strong>Apply</strong>
                        </Typography>
                    </Button>
                );
            case 'pending':
                return (
                    <Button sx={{ display: 'flex', height: '5%' }} variant={'contained'} disabled>
                        <CircularProgress size={20} />
                    </Button>
                );
            default:
                return (
                    <Button sx={{ display: 'flex', height: '5%' }} variant={'contained'} disabled>
                        Failed
                    </Button>
                );
        }
    }
};

export const ActionsCart = () => {
    const [openNotify, setOpenNotify] = React.useState(false);
    const [notifyMessage, setNotifyMessage] = React.useState('');
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

    const closeIcon = (
        <IconButton size="small" aria-label="close" color="secondary" onClick={closeNotification}>
            <Close fontSize="small" />
        </IconButton>
    );

    return (
        <>
            <Drawer
                className="cart-drawer"
                anchor={'right'}
                variant={'permanent'}
                sx={{
                    width: '14%',
                    flexShrink: 0,
                    '& .MuiDrawer-paper': {
                        width: '14%',
                        boxSizing: 'border-box',
                    },
                }}>
                <Paper sx={{ background: theme.palette.primary.main }} square>
                    <Typography variant="h6" align={'center'} color={theme.palette.grey[900]} padding={'3px'}>
                        <strong>Planned Actions</strong>
                    </Typography>
                </Paper>
                <ActionsList openNotification={openNotification} />
            </Drawer>
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
