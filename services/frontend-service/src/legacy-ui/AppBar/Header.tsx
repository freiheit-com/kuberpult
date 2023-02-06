import * as React from 'react';
import Box from '@material-ui/core/Box';
import AppBar from '@material-ui/core/AppBar';
import Typography from '@material-ui/core/Typography';
import { AppDrawer } from './AppDrawer';
import * as api from '../../api/api';
import { LocksDrawer } from './LocksDrawer';
import { ActionsCart } from '../ActionsCart/ActionsCart';
import Tooltip from '@material-ui/core/Tooltip';

export const HeaderTitle: React.FC<any> = (props: { kuberpultVersion: string }) => {
    const { kuberpultVersion } = props;
    return (
        <Tooltip title={`Kuberpult ${kuberpultVersion || ''}`}>
            <code data-testid={'kuberpult-version'}>KUBERPULT UI</code>
        </Tooltip>
    );
};

const Header: React.FC<any> = (props: {
    overview: api.GetOverviewResponse;
    configs: api.GetFrontendConfigResponse;
}) => {
    const { overview, configs } = props;
    return (
        <AppBar>
            <Box sx={{ display: 'flex' }}>
                <Typography component="h1" variant="h6" color="inherit" noWrap sx={{ flexGrow: 1, width: '24rem' }}>
                    <strong>
                        <HeaderTitle kubepultVersion={configs.kuberpultVersion} />
                    </strong>
                </Typography>
                <AppDrawer data={overview} />
                <LocksDrawer data={overview} />
                <ActionsCart overview={overview} />
            </Box>
        </AppBar>
    );
};

export default Header;
