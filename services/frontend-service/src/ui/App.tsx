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
// import logo from '../assets/logo.svg';

import Box from '@material-ui/core/Box';
import AppBar from '@material-ui/core/AppBar';
// import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';

import { makeStyles, createTheme, ThemeProvider, ThemeOptions } from '@material-ui/core/styles';

import Releases from './Releases';
import { GrpcProvider, useObservable } from './Api';

import * as api from '../api/api';
import { EnvironmentLocksDrawer } from './AppDrawer';

export const theme = createTheme({
    palette: {
        mode: 'dark',
        primary: {
            main: '#b9ff00',
            contrastText: '#0c423f',
        },
        secondary: {
            main: '#ff00ff',
            contrastText: '#e9e9e9',
        },
        error: {
            main: '#ff4035',
        },
        success: {
            main: '#00c150',
        },
    },
    typography: {
        subtitle1: {
            color: '#c9c9c9',
        },
    },
} as ThemeOptions);

const useStyles = makeStyles((theme) => ({
    '@global': {
        body: {
            backgroundColor: theme.palette.background.default,
        },
    },
    main: {
        backgroundColor: theme.palette.background.default,
        color: theme.palette.text.primary,
        height: '100vh',
        width: '100vw',
        overflow: 'auto',
        '& > *': {
            height: '100vh',
            paddingTop: '48px',
        },
    },
}));

const GetOverview = (props: { children: (r: api.GetOverviewResponse) => JSX.Element }): JSX.Element | null => {
    const getOverview = React.useCallback((api) => api.overviewService().StreamOverview({}), []);
    const overview = useObservable<api.GetOverviewResponse>(getOverview);
    switch (overview.state) {
        case 'resolved':
            return props.children(overview.result);
        case 'rejected':
            return <div>Error: {JSON.stringify(overview.error)}</div>;
        default:
            return null;
    }
};

const Main = () => {
    const classes = useStyles();
    return (
        <GetOverview>
            {(overview) => (
                <Box sx={{ display: 'flex' }}>
                    <AppBar>
                        <Box sx={{ display: 'flex' }}>
                            <Typography
                                component="h1"
                                variant="h6"
                                color="inherit"
                                noWrap
                                sx={{ flexGrow: 1, width: '10rem' }}>
                                <strong>
                                    <code>KUBERPULT UI</code>
                                </strong>
                            </Typography>
                            <EnvironmentLocksDrawer data={overview} />
                        </Box>
                    </AppBar>
                    <Box component="main" className={classes.main}>
                        <Releases data={overview} />
                    </Box>
                </Box>
            )}
        </GetOverview>
    );
};

export const App: React.FC = () => (
    <ThemeProvider theme={theme}>
        <GrpcProvider>
            <Main />
        </GrpcProvider>
    </ThemeProvider>
);
