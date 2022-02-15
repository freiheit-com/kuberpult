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

import Box from '@material-ui/core/Box';
import { ThemeProvider } from '@material-ui/core/styles';
import CircularProgress from '@material-ui/core/CircularProgress';

import Releases from '../Releases';
import * as api from '../../api/api';
import Header from '../AppBar/Header';
import { GrpcProvider, useObservable } from '../Api';

import { useStyles, theme } from './styles';
import { BatchAction } from '../../api/api';
import { useState } from 'react';

type ActionsCartContextType = {
    actions: BatchAction[];
    setActions: React.Dispatch<React.SetStateAction<BatchAction[]>>;
};
export const ActionsCartContext = React.createContext<ActionsCartContextType>({} as any);

export const Spinner: React.FC<any> = (props: any) => {
    const classes = useStyles();
    return (
        <div className={classes.loading}>
            <CircularProgress size={81} />
        </div>
    );
};

const GetOverview = (props: { children: (r: api.GetOverviewResponse) => JSX.Element }): JSX.Element | null => {
    const getOverview = React.useCallback((api) => api.overviewService().StreamOverview({}), []);
    const overview = useObservable<api.GetOverviewResponse>(getOverview);
    switch (overview.state) {
        case 'resolved':
            return props.children(overview.result);
        case 'rejected':
            return <div>Error: {JSON.stringify(overview.error)}</div>;
        case 'pending':
            return <Spinner />;
        default:
            return null;
    }
};

const Main = () => {
    const classes = useStyles();
    const [actions, setActions] = useState([] as BatchAction[]);
    const value = { actions, setActions };
    return (
        <ActionsCartContext.Provider value={value}>
            <GetOverview>
                {(overview) => (
                    <Box sx={{ display: 'flex' }}>
                        <Header overview={overview} />
                        <Box component="main" className={classes.main}>
                            <Releases data={overview} />
                        </Box>
                    </Box>
                )}
            </GetOverview>
        </ActionsCartContext.Provider>
    );
};

export const App: React.FC = () => (
    <ThemeProvider theme={theme}>
        <GrpcProvider>
            <Main />
        </GrpcProvider>
    </ThemeProvider>
);
