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
import React, { useCallback, useEffect } from 'react';
import Badge from '@material-ui/core/Badge';
import WarningRoundedIcon from '@material-ui/icons/WarningRounded';
import ExpandMoreRounded from '@material-ui/icons/ExpandMoreRounded';
import LockIcon from '@material-ui/icons/Lock';
import { Button, Box, Drawer, Grid, Paper, Typography, Divider, List } from '@material-ui/core';

import { GetOverviewResponse, Commit } from '../../api/api';
import { theme } from '../App/styles';

import { useStyles } from './styles';

interface Locks {
    id: string;
    message: string;
    commit?: Commit;
    type: string;
    age: number;
}

const AllLocks = (props: { locks: Locks[]; onClick: (event: React.KeyboardEvent | React.MouseEvent) => void }) => {
    const { locks, onClick } = props;
    const classes = useStyles();

    const calcLockAge = useCallback((age: number): string => {
        if (age <= 1) return `${age === 0 ? '< 1' : '1'} day ago`;
        return `${age} days ago`;
    }, []);

    if (locks.length === 0) {
        return (
            <Typography noWrap variant="h5" component="div" className="name" width="100%" align="center">
                {'No locks!'}
            </Typography>
        );
    }

    return (
        <List className={classes.environments} sx={{ width: 'auto' }}>
            {locks.map((lock) => {
                const isLockOld = lock.age > 2 || lock.age === -1;
                const Warning = isLockOld ? <WarningRoundedIcon color="error" /> : <LockIcon color="primary" />;
                const lockAge = calcLockAge(lock.age);
                return (
                    <>
                        <Grid item xs={12} key={lock.id} onClick={onClick}>
                            <Paper className="environment">
                                {Warning}
                                <Typography
                                    noWrap
                                    variant="h5"
                                    component="div"
                                    className="name"
                                    width="10%"
                                    align="center">
                                    {lockAge}
                                </Typography>
                                <Typography noWrap variant="h5" component="div" className="name" width="10%">
                                    {lock.id}
                                </Typography>
                                <Typography
                                    noWrap
                                    variant="h5"
                                    component="div"
                                    className="name"
                                    width="40%"
                                    sx={{ textTransform: 'capitalize' }}>
                                    {lock.type}
                                </Typography>
                                <Typography
                                    noWrap
                                    variant="h5"
                                    component="div"
                                    className="big-name"
                                    width="70%"
                                    sx={{ textTransform: 'capitalize' }}>
                                    {lock.message}
                                </Typography>
                            </Paper>
                        </Grid>
                        <Divider />
                    </>
                );
            })}
        </List>
    );
};

export const LocksDrawer = (props: { data: GetOverviewResponse }) => {
    const { data } = props;
    const [state, setState] = React.useState({ isOpen: false });
    const [locks, setLocks] = React.useState<Locks[]>([]);
    const [outDatedLocks, setOutDatedLocks] = React.useState<boolean>(false);

    const toggleDrawer = (open: boolean) => (event: React.KeyboardEvent | React.MouseEvent) => {
        if (
            event.type === 'keydown' &&
            ((event as React.KeyboardEvent).key === 'Tab' || (event as React.KeyboardEvent).key === 'Shift')
        ) {
            return;
        }
        setState({ isOpen: open });
    };

    const calcAge = useCallback((time: any) => {
        if (!time) return -1;
        const curTime = new Date().getTime();
        const diffTime = curTime - time;
        const msPerDay = 1000 * 60 * 60 * 24;
        const diffDays = Math.floor(diffTime / msPerDay);
        return diffDays;
    }, []);

    useEffect(() => {
        let nwLocks: Locks[] = [];
        for (const env of Object.values(data.environments)) {
            nwLocks = [
                ...nwLocks,
                ...Object.keys(env.locks ?? {}).map((value) => ({
                    id: value,
                    ...env.locks[value],
                    type: env.name,
                    age: calcAge(env.locks[value].commit?.authorTime),
                })),
            ];

            for (const app of Object.values(env.applications)) {
                nwLocks = [
                    ...nwLocks,
                    ...Object.keys(app.locks ?? {}).map((value) => ({
                        id: value,
                        ...app.locks[value],
                        type: app.name,
                        age: calcAge(app.locks[value].commit?.authorTime),
                    })),
                ];
            }
        }
        nwLocks.sort((a: Locks, b: Locks) => {
            if (!a.commit?.authorTime || !b.commit?.authorTime) return 0;
            if (a.commit.authorTime < b.commit.authorTime) return 1;
            if (a.commit.authorTime === b.commit.authorTime) return 0;
            return -1;
        });
        setLocks(nwLocks);
    }, [data, calcAge]);

    useEffect(() => {
        setOutDatedLocks(locks.filter((lock) => !!lock.age && lock?.age > 2).length > 0);
    }, [locks]);

    return (
        <>
            <Button
                sx={{ color: theme.palette.grey[900], width: '100%' }}
                variant={'contained'}
                onClick={toggleDrawer(true)}>
                <Badge
                    invisible={!outDatedLocks}
                    anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                    badgeContent={<WarningRoundedIcon color="error" />}>
                    <strong>all locks</strong>
                </Badge>
                <ExpandMoreRounded />
            </Button>
            <Drawer anchor={'top'} open={state['isOpen']} onClose={toggleDrawer(false)}>
                <Box sx={{ width: 'auto' }} role="presentation">
                    <AllLocks locks={locks} onClick={toggleDrawer(false)} />
                </Box>
            </Drawer>
        </>
    );
};
