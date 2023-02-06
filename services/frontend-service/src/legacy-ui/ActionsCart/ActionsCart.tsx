import * as React from 'react';
import { useCallback, useContext, VFC } from 'react';
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

import ClearRounded from '@material-ui/icons/ClearRounded';
import { ActionsCartContext } from '../App';
import { GetOverviewResponse } from '../../api/api';
import { theme } from '../App/styles';
import { CheckoutCart } from './CheckoutDialog';
import { CartAction, getActionDetails } from '../ActionDetails';

const ActionListItem = (props: { act: CartAction; index: number }) => {
    const { act, index } = props;
    const { actions, setActions } = useContext(ActionsCartContext);
    const removeItem = useCallback(() => {
        setActions(actions.filter((_, i) => i !== index));
    }, [actions, setActions, index]);

    return (
        <ListItem divider={true}>
            <ListItemAvatar>
                <Avatar>{getActionDetails(act).icon}</Avatar>
            </ListItemAvatar>
            <ListItemText primary={getActionDetails(act).name} secondary={getActionDetails(act).summary} />
            <IconButton onClick={removeItem}>
                <ClearRounded />
            </IconButton>
        </ListItem>
    );
};

const ActionsList: VFC<{ overview: GetOverviewResponse }> = ({ overview }) => {
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
                {actions.map((act, index) => (
                    <ActionListItem act={act} index={index} key={index} />
                ))}
            </List>
            {actions.length === 0 ? (
                <Typography variant="h6" whiteSpace={'pre-line'} align={'center'}>
                    {'Cart Is Currently Empty,\nPlease Add Actions!'}
                </Typography>
            ) : null}
            <CheckoutCart overview={overview} />
        </div>
    );
};

export const ActionsCart: VFC<{ overview: GetOverviewResponse }> = ({ overview }) => (
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
        <ActionsList overview={overview} />
    </Drawer>
);
