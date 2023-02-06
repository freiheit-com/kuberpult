import * as React from 'react';
import { useRef, useState } from 'react';

import Box from '@material-ui/core/Box';
import { ThemeProvider } from '@material-ui/core/styles';
import CircularProgress from '@material-ui/core/CircularProgress';
import { AuthProvider, AuthTokenContext } from './AuthContext';

import Releases from '../Releases';
import * as api from '../../api/api';
import Header from '../AppBar/Header';
import { Context, GrpcProvider, UnaryState, useUnary } from '../Api';

import { theme, useStyles } from './styles';
import { CartAction } from '../ActionDetails';
import refreshStore from './RefreshStore';

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
    const [overview, setOverview] = useState<UnaryState<api.GetOverviewResponse>>({ state: 'pending' });
    const reloadDelayMillis = 30 * 1000;
    const api = React.useContext(Context);
    const { authHeader } = React.useContext(AuthTokenContext);

    const updateOverview = React.useCallback(() => {
        api.overviewService()
            .GetOverview({}, authHeader)
            .then(
                (result) => {
                    setOverview({ result, state: 'resolved' });
                    refreshStore.setRefresh(false);
                },
                (error) => setOverview({ error, state: 'rejected' })
            );
    }, [api, authHeader]);

    React.useEffect(() => {
        const id = setInterval(updateOverview, reloadDelayMillis);
        return () => clearInterval(id);
    }, [reloadDelayMillis, updateOverview]);

    const backupState = useRef<api.GetOverviewResponse>();
    if (backupState.current === undefined || refreshStore.shouldRefresh()) {
        backupState.current = { environments: {}, applications: {} } as api.GetOverviewResponse;
        updateOverview();
    }

    switch (overview.state) {
        case 'resolved':
            backupState.current = overview.result;
            return props.children(backupState.current);
        case 'rejected':
            // eslint-disable-next-line no-console
            console.log('restarting streamoverview due to error: ', overview.error);
            return props.children(backupState.current);
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

    const getConfig = React.useCallback((api: any) => api.configService().GetConfig(), []);
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
                                <Header overview={overview} configs={configs} />
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
