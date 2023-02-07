/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/
import * as React from 'react';

import ExpandMoreRounded from '@material-ui/icons/ExpandMoreRounded';
import { Button, Box, Drawer, Grid, Paper, Typography, Divider, List, ButtonGroup } from '@material-ui/core';

import { GetOverviewResponse, Lock } from '../../api/api';
import { theme } from '../App/styles';
import { calculateDistanceToUpstream, sortEnvironmentsByUpstream } from '../Releases';
import { CreateLockButton, ReleaseLockButton } from '../ReleaseDialog';

import { useStyles } from './styles';

const EnvironmentLocks = (props: { data: GetOverviewResponse }) => {
    const { data } = props;
    const classes = useStyles(data.environments);
    const sortOrder = calculateDistanceToUpstream(Object.values(data.environments));
    const envLocks: { [index: string]: [string, Lock][] } = {};
    for (const env of Object.values(data.environments)) {
        envLocks[env.name] = Object.entries(data.environments[env.name].locks ?? {});
        envLocks[env.name].sort((a: [string, Lock], b: [string, Lock]) => {
            if (a[0] < b[0]) return -1;
            if (a[0] === b[0]) return 0;
            return 1;
        });
    }
    const sortedEnvs = sortEnvironmentsByUpstream(Object.values(data.environments), sortOrder);

    return (
        <List className={classes.environments} sx={{ width: 'auto' }}>
            {sortedEnvs.map((env) => (
                <>
                    <Grid item xs={12} key={env.name}>
                        <Paper className="environment">
                            <Typography
                                noWrap
                                variant="h5"
                                component="div"
                                className="name"
                                width="30%"
                                sx={{ textTransform: 'capitalize' }}>
                                {env.name}
                            </Typography>
                            <ButtonGroup className="locks">
                                <CreateLockButton environmentName={env.name} />
                                {envLocks[env.name].map(([key, lock]) => (
                                    <ReleaseLockButton environmentName={env.name} lockId={key} lock={lock} />
                                ))}
                            </ButtonGroup>
                        </Paper>
                    </Grid>
                    <Divider />
                </>
            ))}
        </List>
    );
};

export const AppDrawer = (props: { data: GetOverviewResponse }) => {
    const { data } = props;
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
            <Button
                sx={{ color: theme.palette.grey[900], width: '100%' }}
                variant={'contained'}
                onClick={toggleDrawer(true)}>
                <strong>Environment</strong>
                <ExpandMoreRounded />
            </Button>
            <Drawer anchor={'top'} open={state['isOpen']} onClose={toggleDrawer(false)}>
                <Box sx={{ width: 'auto' }} role="presentation">
                    <EnvironmentLocks data={data} />
                </Box>
                <Button onClick={toggleDrawer(false)}>Close</Button>
            </Drawer>
        </>
    );
};
