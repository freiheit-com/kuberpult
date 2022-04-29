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
    AppBar,
    Avatar,
    Drawer,
    IconButton,
    List,
    ListItem,
    ListItemAvatar,
    ListItemText,
    Typography,
} from '@material-ui/core';

import { ClearRounded } from '@material-ui/icons';
import { ActionsCartContext } from '../App';
import { BatchAction } from '../../api/api';
import { theme } from '../App/styles';
import { CheckoutCart } from './CheckoutDialog';
import { GetActionDetails } from '../ActionDetails';

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

const ActionsList = () => {
    const { actions } = useContext(ActionsCartContext);

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
                paddingTop: '30px',
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
            <CheckoutCart />
        </div>
    );
};

export const ActionsCart = () => (
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
        <AppBar sx={{ width: 'inherit' }}>
            <Typography variant="h6" align={'center'} noWrap color={theme.palette.grey[900]} padding={'3px'}>
                <strong>{'Planned Actions'}</strong>
            </Typography>
        </AppBar>
        <ActionsList />
    </Drawer>
);
