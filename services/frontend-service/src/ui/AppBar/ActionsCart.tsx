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

import {
    Avatar,
    Box,
    Button,
    Drawer,
    IconButton,
    List,
    ListItem,
    ListItemAvatar,
    ListItemText,
} from '@material-ui/core';

import { theme } from '../App/styles';
import { ClearRounded, PlaylistAddCheck } from '@material-ui/icons';
import { useCallback, useContext } from 'react';
import { ActionsCartContext } from '../App';
import { BatchAction } from '../../api/api';
import { callbacks, GetActionDetails } from '../Batch';
import Typography from '@material-ui/core/Typography';

const ActionListItem = (props: { act: BatchAction; index: number }) => {
    const { act, index } = props;
    const { actions, setActions } = useContext(ActionsCartContext);
    const removeItem = useCallback(() => {
        setActions([...actions.slice(0, index), ...actions.slice(index + 1)]);
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

const ActionsList = () => {
    const { actions, setActions } = useContext(ActionsCartContext);
    const clearList = useCallback(() => {
        setActions([]);
    }, [setActions]);
    const [doActions] = callbacks.useBatch(actions, clearList);

    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                height: '100vh',
            }}>
            <List className="actions" sx={{ width: '100%', maxWidth: 360, bgcolor: 'background.paper' }}>
                {actions.map((act: BatchAction, index: number) => (
                    <ActionListItem act={act} index={index} />
                ))}
            </List>
            {actions.length === 0 ? (
                <Typography variant="h6" whiteSpace={'pre-line'} align={'center'} padding={'20px'}>
                    {'Cart Is Currently Empty,\nPlease Add Actions!'}
                </Typography>
            ) : null}
            <Button
                sx={{ display: 'flex', height: '5%' }}
                onClick={doActions}
                variant={'contained'}
                disabled={actions.length === 0}>
                <Typography variant="h6">
                    <strong>Apply</strong>
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
