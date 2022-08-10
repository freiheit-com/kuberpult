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
import { useState } from 'react';

import Box from '@material-ui/core/Box';
import { ThemeProvider } from '@material-ui/core/styles';
import CircularProgress from '@material-ui/core/CircularProgress';
import { AuthProvider, AuthTokenContext } from './AuthContext';

import Releases from '../Releases';
import * as api from '../../api/api';
import Header from '../AppBar/Header';
import { GrpcProvider, useObservable, useUnary } from '../Api';

import { theme, useStyles } from './styles';
import { CartAction } from '../ActionDetails';

type ConfigContextType = {
    configs: api.GetFrontendConfigResponse;
    setConfigs: React.Dispatch<React.SetStateAction<api.GetFrontendConfigResponse>>;
};
export const ConfigsContext = React.createContext<ConfigContextType>({} as any);

type ActionsCartContextType = {
    actions: CartAction[];
    setActions: React.Dispatch<React.SetStateAction<CartAction[]>>;
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
    const getOverview = React.useCallback(
        (api, authHeader) => api.overviewService().StreamOverview({}, authHeader),
        []
    );
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
    const [actions, setActions] = useState([] as CartAction[]);
    const [configs, setConfigs] = useState({} as api.GetFrontendConfigResponse);

    const getConfig = React.useCallback((api) => api.configService().GetConfig(), []);
    const config = useUnary<api.GetFrontendConfigResponse>(getConfig);

    React.useEffect(() => {
        if (config.state === 'resolved') {
            setConfigs(config.result);
        }
    }, [config]);

    return (
        <ActionsCartContext.Provider value={{ actions, setActions }}>
            <ConfigsContext.Provider value={{ configs, setConfigs }}>
                <AuthProvider>
                    <GetOverview>
                        {(overview) => (
                            <Box sx={{ display: 'flex', marginRight: '14%' }}>
                                <Header overview={overview} />
                                <Box component="main" className={classes.main}>
                                    <Releases data={overview} />
                                </Box>
                            </Box>
                        )}
                    </GetOverview>
                </AuthProvider>
            </ConfigsContext.Provider>
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
