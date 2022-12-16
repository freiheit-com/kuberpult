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
import { Dialog, Tooltip } from '@material-ui/core';
import classNames from 'classnames';
import React from 'react';
import { Application, Environment } from '../../../api/api';
import { Locks, LocksWhite } from '../../../images';
import { updateReleaseDialog, useApplicationLocks, useCurrentlyDeployedAt, useReleaseInfo } from '../../utils/store';
import { Button } from '../button';

export type ReleaseDialogProps = {
    className?: string;
    app: Application;
    version: number;
};

type EnvSortOrder = { [index: string]: number };

const setClosed = () => {
    updateReleaseDialog({} as Application, 0);
};

const sortEnvironmentsByUpstream = (envs: Environment[]): Environment[] => {
    const sortedEnvs = [...envs];
    const distance = calculateDistanceToUpstream(envs);
    sortedEnvs.sort((a: Environment, b: Environment) => {
        if (distance[a.name] === distance[b.name]) {
            if (a.name < b.name) return -1;
            if (a.name === b.name) return 0;
            return 1;
        }
        if (distance[a.name] < distance[b.name]) return -1;
        return 1;
    });
    return sortedEnvs.reverse();
};

export const calculateDistanceToUpstream = (envs: Environment[]): EnvSortOrder => {
    const distanceToUpstream: EnvSortOrder = {};
    let rest: Environment[] = [];
    for (const env of envs) {
        if (!env.config?.upstream?.upstream?.$case || env.config?.upstream?.upstream?.$case === 'latest') {
            distanceToUpstream[env.name] = 0;
        } else {
            rest.push(env);
        }
    }
    // iterate over rest until nothing is left
    while (rest.length > 0) {
        const nextRest: Environment[] = [];
        for (const env of rest) {
            const upstreamEnv = (env.config?.upstream?.upstream as any).environment;
            if (upstreamEnv in distanceToUpstream) {
                distanceToUpstream[env.name] = distanceToUpstream[upstreamEnv] + 1;
            } else {
                nextRest.push(env);
            }
        }
        if (rest.length === nextRest.length) {
            // infinite loop here, maybe fill in the remaining entries with max(distanceToUpstream) + 1
            for (const env of rest) {
                distanceToUpstream[env.name] = envs.length + 1;
            }
            return distanceToUpstream;
        }
        rest = nextRest;
    }
    return distanceToUpstream;
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const { className, app, version } = props;
    const appLocks = useApplicationLocks().filter((lock) => lock.application === app.name);
    const envs = useCurrentlyDeployedAt(app, version);
    const release = useReleaseInfo(app, version);

    const dialog =
        app.name !== ({} as Application).name ? (
            <div>
                <Dialog
                    className={classNames('release-dialog', className)}
                    fullWidth={true}
                    maxWidth="md"
                    open={app.name !== ({} as Application).name}
                    onClose={setClosed}>
                    <div className={classNames('release-dialog-app-bar', className)}>
                        <div className={classNames('release-dialog-app-bar-data')}>
                            <div className={classNames('release-dialog-message', className)}>
                                <span className={classNames('release-dialog-commitMessage', className)}>
                                    {release?.sourceMessage}
                                </span>
                            </div>
                            <div className={classNames('release-dialog-createdAt', className)}>
                                {!!release?.createdAt && (
                                    <div>
                                        {'Release date ' +
                                            release?.createdAt.toISOString().split('T')[0] +
                                            ' ' +
                                            release?.createdAt.toISOString().split('T')[1].split(':')[0] +
                                            ':' +
                                            release?.createdAt.toISOString().split('T')[1].split(':')[1]}
                                    </div>
                                )}
                            </div>
                            <div className={classNames('release-dialog-author', className)}>
                                {release?.sourceAuthor ? 'Author:' + release?.sourceAuthor : ''}
                            </div>
                        </div>
                        <span className={classNames('release-dialog-commitId', className)}>
                            {release.undeployVersion === undefined ? 'undeploy version' : release?.sourceCommitId}
                        </span>
                        <Button
                            onClick={setClosed}
                            className={classNames('release-dialog-close', className)}
                            icon={
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
                            }
                        />
                    </div>
                    <ul className={classNames('release-env-list', className)}>
                        {sortEnvironmentsByUpstream(envs).map((env) => (
                            <li key={env.name} className={classNames('env-card', className)}>
                                <div className="env-card-header">
                                    <div className={classNames('env-card-label')}>
                                        <div>{env.name}</div>

                                        {Object.values(env.locks).map((lock) => (
                                            <Tooltip
                                                arrow
                                                title={
                                                    'Lock Message: "' +
                                                    lock.message +
                                                    '" | ID: "' +
                                                    lock.lockId +
                                                    '"  | Click to unlock. '
                                                }>
                                                <div>
                                                    <Button
                                                        icon={<LocksWhite className="env-card-env-lock" />}
                                                        className={'button-lock'}
                                                    />
                                                </div>
                                            </Tooltip>
                                        ))}
                                    </div>

                                    <div className={classNames('env-card-app-locks')}>
                                        {appLocks
                                            .filter((lock) => lock.environment === env.name)
                                            .map((lock) => (
                                                <Tooltip
                                                    arrow
                                                    title={
                                                        'Lock Message: "' +
                                                        lock.message +
                                                        '" | ID: "' +
                                                        lock.lockId +
                                                        '"  | Click to unlock. '
                                                    }>
                                                    <div>
                                                        <Button icon={<LocksWhite />} className={'button-lock'} />
                                                    </div>
                                                </Tooltip>
                                            ))}
                                    </div>
                                </div>
                                <div className={classNames('env-card-data', className)}>
                                    {release.sourceCommitId}:{release.sourceMessage}
                                </div>
                                <div className="env-card-buttons">
                                    <Button
                                        className="env-card-add-lock-btn"
                                        label="Add lock"
                                        icon={<Locks className="icon" />}
                                    />
                                    <Button className="env-card-deploy-btn" label="Deploy" />
                                </div>
                            </li>
                        ))}
                    </ul>
                </Dialog>
            </div>
        ) : (
            ''
        );

    return <div>{dialog}</div>;
};
