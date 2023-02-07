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
import React, { useCallback } from 'react';
import ExpandMoreRounded from '@material-ui/icons/ExpandMoreRounded';
import {
    Button,
    Box,
    Drawer,
    Paper,
    TableHead,
    TableRow,
    TableCell,
    TableSortLabel,
    TableContainer,
    Table,
    TableBody,
    Tooltip,
    Typography,
} from '@material-ui/core';

import { GetOverviewResponse } from '../../api/api';
import { theme } from '../App/styles';
import WarningRounded from '@material-ui/icons/WarningRounded';

interface LocksRow {
    id: string;
    author: string;
    authorEmail: string;
    message: string;
    dateAdded: number;
    environment: string;
    application: string;
}

function descendingComparator<T>(a: T, b: T, orderBy: keyof T) {
    if (b[orderBy] < a[orderBy]) {
        return -1;
    }
    if (b[orderBy] > a[orderBy]) {
        return 1;
    }
    return 0;
}

type Order = 'asc' | 'desc';

function getComparator<Key extends keyof any>(
    order: Order,
    orderBy: Key
): (a: { [key in Key]: string | number }, b: { [key in Key]: string | number }) => number {
    return order === 'desc'
        ? (a, b) => descendingComparator(a, b, orderBy)
        : (a, b) => -descendingComparator(a, b, orderBy);
}

interface HeaderCell {
    id: keyof LocksRow;
    label: string;
}

const headerCells: readonly HeaderCell[] = [
    {
        id: 'dateAdded',
        label: 'Date Added',
    },
    {
        id: 'environment',
        label: 'Environment',
    },
    {
        id: 'application',
        label: 'Application',
    },
    {
        id: 'author',
        label: 'Author',
    },
    {
        id: 'message',
        label: 'Message',
    },
];

interface LocksTableHeaderProps {
    onRequestSort: (event: React.MouseEvent<unknown>, property: keyof LocksRow) => void;
    order: Order;
    orderBy: string;
    type: 'env' | 'app';
}

const LocksTableHeader = (props: LocksTableHeaderProps) => {
    const { order, orderBy, onRequestSort, type } = props;
    const createSortHandler = (property: keyof LocksRow) => (event: React.MouseEvent<unknown>) => {
        onRequestSort(event, property);
    };

    return (
        <TableHead>
            <TableRow>
                <TableCell align={'center'} padding={'normal'} colSpan={5}>
                    <Typography
                        variant="h6"
                        component="div"
                        className="locks-table-header-name"
                        sx={{ color: theme.palette.primary.light }}>
                        {type === 'env' ? 'Environment Locks' : 'Application Locks'}
                    </Typography>
                </TableCell>
            </TableRow>
            <TableRow>
                {headerCells.map((headerCell) =>
                    headerCell.id === 'application' && type === 'env' ? null : (
                        <TableCell
                            key={headerCell.id}
                            padding={'normal'}
                            sortDirection={orderBy === headerCell.id ? order : false}>
                            <TableSortLabel
                                active={orderBy === headerCell.id}
                                direction={orderBy === headerCell.id ? order : 'asc'}
                                onClick={createSortHandler(headerCell.id)}>
                                <strong>{headerCell.label}</strong>
                            </TableSortLabel>
                        </TableCell>
                    )
                )}
            </TableRow>
        </TableHead>
    );
};

const getFullDate = (time: number) => {
    // use -1 to sort the dates with the newest on top
    time *= -1;
    if (time === -1) return '';
    const d = new Date(time);
    return d.toString();
};

const daysToString = (days: number) => {
    if (days === -1) return '';
    if (days === 0) return '< 1 day ago';
    if (days === 1) return '1 day ago';
    return `${days} days ago`;
};

const calcLockAge = (time: number): number => {
    // use -1 to sort the dates with the newest on top
    time *= -1;
    if (time === -1) return -1;
    const curTime = new Date().getTime();
    const diffTime = curTime.valueOf() - time;
    const msPerDay = 1000 * 60 * 60 * 24;
    return Math.floor(diffTime / msPerDay);
};

const isOutdated = (dateAdded: number): boolean => calcLockAge(dateAdded) > 2 || dateAdded === -1;

const AllLocks = (props: { locks: LocksRow[]; type: 'env' | 'app' }) => {
    const { locks, type } = props;
    const [order, setOrder] = React.useState<Order>('asc');
    const [orderBy, setOrderBy] = React.useState<keyof LocksRow>('dateAdded');

    const handleRequestSort = useCallback(
        (event: React.MouseEvent<unknown>, property: keyof LocksRow) => {
            const isAsc = orderBy === property && order === 'asc';
            setOrder(isAsc ? 'desc' : 'asc');
            setOrderBy(property);
        },
        [order, orderBy, setOrder, setOrderBy]
    );

    return (
        <Paper sx={{ width: '100%', mb: 2 }}>
            <TableContainer>
                <Table sx={{ minWidth: 750 }} aria-labelledby="locksTableTitle" size={'small'}>
                    <LocksTableHeader order={order} orderBy={orderBy} onRequestSort={handleRequestSort} type={type} />
                    <TableBody>
                        {locks
                            .slice()
                            .sort(getComparator(order, orderBy))
                            .map((row, index) => (
                                <TableRow hover key={row.id}>
                                    <TableCell component="th" id={`locks-table-${index}`} scope="row">
                                        <Tooltip title={getFullDate(row.dateAdded)} placement={'top-start'}>
                                            <Typography color={isOutdated(row.dateAdded) ? 'error' : ''}>
                                                {daysToString(calcLockAge(row.dateAdded))}
                                            </Typography>
                                        </Tooltip>
                                    </TableCell>
                                    <TableCell>{row.environment}</TableCell>
                                    {type === 'app' && <TableCell>{row.application}</TableCell>}
                                    <TableCell>
                                        <Tooltip title={row.authorEmail} placement={'left-end'}>
                                            <Typography>{row.author}</Typography>
                                        </Tooltip>
                                    </TableCell>
                                    <TableCell>
                                        <Tooltip title={row.id} placement={'left-end'}>
                                            <Typography>{row.message}</Typography>
                                        </Tooltip>
                                    </TableCell>
                                </TableRow>
                            ))}
                        {!locks.length && (
                            <TableRow>
                                <TableCell align={'center'} padding={'normal'} colSpan={5}>
                                    <Typography variant="subtitle1" component="div" className="locks-table-empty">
                                        There are no locks here yet!
                                    </Typography>
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </Table>
            </TableContainer>
        </Paper>
    );
};

export const LocksDrawer = (props: { data: GetOverviewResponse }) => {
    const { data } = props;
    const [drawerOpen, setDrawerOpen] = React.useState(false);

    const toggleDrawer = (open: boolean) => (event: React.KeyboardEvent | React.MouseEvent) => {
        if (
            event.type === 'keydown' &&
            ((event as React.KeyboardEvent).key === 'Tab' || (event as React.KeyboardEvent).key === 'Shift')
        ) {
            return;
        }
        setDrawerOpen(open);
    };

    const envLocks: LocksRow[] = [];
    const appLocks: LocksRow[] = [];

    for (const env of Object.values(data.environments)) {
        envLocks.push(
            ...Object.entries(env.locks ?? {}).map((item) => ({
                id: item[0],
                author: item[1].createdBy?.name ?? '',
                authorEmail: item[1].createdBy?.email ?? '',
                message: item[1].message,
                // use -1 to sort the locks with the newest on top
                dateAdded: (item[1].createdAt?.valueOf() ?? -1) * -1,
                environment: env.name,
                application: '',
            }))
        );

        for (const app of Object.values(env.applications)) {
            appLocks.push(
                ...Object.entries(app.locks ?? {}).map((item) => ({
                    id: item[0],
                    author: item[1].createdBy?.name ?? '',
                    authorEmail: item[1].createdBy?.email ?? '',
                    message: item[1].message,
                    // use -1 to sort the locks with the newest on top
                    dateAdded: (item[1].createdAt?.valueOf() ?? -1) * -1,
                    environment: env.name,
                    application: app.name,
                }))
            );
        }
    }

    const outdated =
        envLocks.filter((lock) => isOutdated(lock.dateAdded)).length > 0 ||
        appLocks.filter((lock) => isOutdated(lock.dateAdded)).length > 0;

    return (
        <>
            <Button
                sx={{ color: theme.palette.grey[900], width: '100%' }}
                variant={'contained'}
                onClick={toggleDrawer(true)}>
                {outdated && <WarningRounded color="error" />}
                <strong>Locks</strong>
                <ExpandMoreRounded />
            </Button>
            <Drawer anchor={'top'} open={drawerOpen} onClose={toggleDrawer(false)}>
                <Box sx={{ width: 'auto' }} role="presentation">
                    <AllLocks locks={envLocks} type={'env'} />
                    <AllLocks locks={appLocks} type={'app'} />
                </Box>
                <Button onClick={toggleDrawer(false)}>Close</Button>
            </Drawer>
        </>
    );
};
