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
import { AppBar, Button, Dialog, IconButton, List, ListItem, ListItemText, Toolbar } from '@material-ui/core';
import React from 'react';
import { useCurrentReleaseDialog, updateReleaseDialog, useCurrentlyDeployedAt } from '../../utils/store';

export type ReleaseDialogProps = {
    className?: string;
};

const setClosed = () => {
    updateReleaseDialog(false, '', 0);
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const release = useCurrentReleaseDialog();
    const app = release[0];
    const version = release[1];

    const envs = useCurrentlyDeployedAt(app, version);

    const dialog =
        app !== '' ? (
            <div>
                <Dialog fullScreen={false} open={app !== ''} onClose={setClosed}>
                    {app} {version}
                    <AppBar sx={{ position: 'relative' }}>
                        <Toolbar>
                            <IconButton
                                edge="start"
                                color="inherit"
                                onClick={setClosed}
                                aria-label="close"></IconButton>
                            <Button autoFocus color="inherit" onClick={setClosed}>
                                close
                            </Button>
                        </Toolbar>
                    </AppBar>
                    <List>
                        {envs.map((env) => (
                            <ListItem button>
                                <ListItemText primary={env} />
                            </ListItem>
                        ))}
                    </List>
                </Dialog>
            </div>
        ) : (
            <div />
        );

    return <div>{dialog}</div>;
};
