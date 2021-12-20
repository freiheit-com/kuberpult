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
import AppBar from '@material-ui/core/AppBar';
import Typography from '@material-ui/core/Typography';
import { AppDrawer } from './AppDrawer';
import * as api from '../../api/api';
import { LocksDrawer } from './LocksDrawer';

export const Header: React.FC<any> = (props: { overview: api.GetOverviewResponse }) => {
    const { overview } = props;
    return (
        <AppBar>
            <Box sx={{ display: 'flex' }}>
                <Typography component="h1" variant="h6" color="inherit" noWrap sx={{ flexGrow: 1, width: '12rem' }}>
                    <strong>
                        <code>KUBERPULT UI</code>
                    </strong>
                </Typography>
                <AppDrawer data={overview} />
                <LocksDrawer data={overview} />
            </Box>
        </AppBar>
    );
};
