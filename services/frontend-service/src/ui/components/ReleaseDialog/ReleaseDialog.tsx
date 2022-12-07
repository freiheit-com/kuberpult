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
import { AppBar, Button, Dialog, List, ListItem, ListItemText } from '@material-ui/core';
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
    updateReleaseDialog('', 0);
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const dialog =
        props.version !== -1 ? (
            <div>
                <Dialog
                    className={classNames('release-dialog', props.className)}
                    fullWidth={true}
                    maxWidth="md"
                    open={props.app !== ''}
                    onClose={setClosed}>
                    <AppBar className={classNames('release-dialog', props.className)} sx={{ position: 'relative' }}>
                        <span className={classNames('release-dialog-close', props.className)}>
                            <Button onClick={setClosed}>
                                <svg
                                    width="20"
                                    height="20"
                                    viewBox="0 0 20 20"
                                    fill="none"
                                    xmlns="http://www.w3.org/2000/svg">
                                    <path
                                        d="M1 1L19 19M19 1L1 19"
                                        stroke="white"
                                        strokeWidth="2"
                                        strokeLinecap="round"
                                    />
                                </svg>
                            </Button>
                        </span>
                        <div className={classNames('release-dialog-message', props.className)}>
                            {props.release.sourceMessage}
                            <span className={classNames('release-dialog-commitId', props.className)}>
                                {props.release.sourceCommitId}
                            </span>
                        </div>
                        <div className={classNames('release-dialog-createdAt', props.className)}>
                            {!!props.release.createdAt && (
                                <div>{'Release date ' + props.release.createdAt.toISOString()}</div>
                            )}
                        </div>
                        <div className={classNames('release-dialog-author', props.className)}>
                            Author: {props.release.sourceAuthor}
                        </div>
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
            ''
        );

    return <div>{dialog}</div>;
};
