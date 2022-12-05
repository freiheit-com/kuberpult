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
import classNames from 'classnames';
import React from 'react';
import { updateReleaseDialog } from '../../utils/store';

export type ReleaseDialogProps = {
    className?: string;
    app: string;
    version: number;
    release: object;
    envs: Array<string>;
};

const setClosed = () => {
    updateReleaseDialog(false, '', 0);
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const dialog =
        props.version !== -1 ? (
            <div>
                <Dialog fullWidth={true} maxWidth="md" open={props.app !== ''} onClose={setClosed}>
                    {props.app} {props.version}
                    <AppBar sx={{ position: 'relative' }}>
                        <div className={classNames('release-dialog-message', props.className)}>
                            {props.release.sourceMessage}
                            <span className={classNames('release-dialog-commitId', props.className)}>
                                {props.release.sourceCommitId}
                            </span>
                        </div>
                        <div className={classNames('release-dialog-author', props.className)}>
                            Author: {props.release.sourceAuthor}
                        </div>

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
                        {props.envs.map((env) => (
                            <ListItem button key={env}>
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
