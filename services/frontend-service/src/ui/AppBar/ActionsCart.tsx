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

import { Avatar, Box, Button, Drawer, List, ListItem, ListItemAvatar, ListItemText } from '@material-ui/core';

import { theme } from '../App/styles';
import {
    DeleteForeverRounded,
    DeleteOutlineRounded,
    LockOpenRounded,
    LockRounded,
    MoveToInboxRounded,
    PlaylistAddCheck,
} from '@material-ui/icons';
import { useContext } from 'react';
import { ActionsCartContext } from '../App';
import { BatchAction } from '../../api/api';
import { callbacks } from '../Batch';
import Typography from '@material-ui/core/Typography';

const ActionsList = () => {
    const { actions } = useContext(ActionsCartContext);
    const [doActions] = callbacks.useBatch(actions);

    if (actions.length === 0) {
        return <div>Cart Empty</div>;
    }
    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                height: '100vh',
            }}>
            <List className="actions" sx={{ width: '100%', maxWidth: 360, bgcolor: 'background.paper' }}>
                {actions.map((act: BatchAction) => (
                    <ListItem>
                        <ListItemAvatar>
                            <Avatar>{getListAction(act).icon}</Avatar>
                        </ListItemAvatar>
                        <ListItemText primary={getListAction(act).name} secondary={getListAction(act).message} />
                    </ListItem>
                ))}
            </List>
            <Button sx={{ display: 'flex', height: '5%' }} onClick={doActions} variant={'contained'}>
                <Typography variant="h6">
                    <strong>Checkout</strong>
                </Typography>
            </Button>
        </div>
    );
};

export const ActionsCart = () => {
    const [state, setState] = React.useState({ isOpen: false });
    const toggleDrawer = (open: boolean) => (event: React.KeyboardEvent | React.MouseEvent) => {
        if (
            event.type === 'keydown' &&
            ((event as React.KeyboardEvent).key === 'Tab' || (event as React.KeyboardEvent).key === 'Shift')
        ) {
            return;
        }
        setState({ isOpen: open });
    };

    return (
        <>
            <Button sx={{ color: theme.palette.grey[900] }} onClick={toggleDrawer(true)} variant={'contained'}>
                <PlaylistAddCheck />
            </Button>
            <Drawer anchor={'right'} open={state['isOpen']} onClose={toggleDrawer(false)}>
                <Box sx={{ width: 'auto' }} role="presentation">
                    <ActionsList />
                </Box>
            </Drawer>
        </>
    );
};

type ListAction = {
    name: string;
    message: string;
    icon?: React.ReactElement;
};

const getListAction = (action: BatchAction): ListAction => {
    switch (action.action?.$case) {
        case 'deploy':
            return {
                name: 'Deploy',
                message: 'Deploy version ' + action.action?.deploy.version + ' to ' + action.action?.deploy.environment,
                icon: <MoveToInboxRounded />,
            };
        case 'createEnvironmentLock':
            return {
                name: 'Create Env Lock',
                message:
                    'Create new environment lock on ' +
                    action.action?.createEnvironmentLock.environment +
                    '. | Lock Message: ' +
                    action.action?.createEnvironmentLock.message,
                icon: <LockRounded />,
            };
        case 'createEnvironmentApplicationLock':
            return {
                name: 'Create App Lock',
                message:
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
                message: 'Delete environment lock on ' + action.action?.deleteEnvironmentLock.environment,
                icon: <LockOpenRounded />,
            };
        case 'deleteEnvironmentApplicationLock':
            return {
                name: 'Delete App Lock',
                message:
                    'Unlock "' +
                    action.action?.deleteEnvironmentApplicationLock.application +
                    '" on ' +
                    action.action?.deleteEnvironmentApplicationLock.environment,
                icon: <LockOpenRounded />,
            };
        case 'prepareUndeploy':
            return {
                name: 'Prepare Undeploy',
                message: 'Prepare undeploy version for Application ' + action.action?.prepareUndeploy.application,
                icon: <DeleteOutlineRounded />,
            };
        case 'undeploy':
            return {
                name: 'Undeploy',
                message: 'Undeploy and delete Application ' + action.action?.undeploy.application,
                icon: <DeleteForeverRounded />,
            };
        default:
            return {
                name: 'invalid',
                message: 'invalid',
            };
    }
};
