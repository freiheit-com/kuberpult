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
import { Environment, Release } from '../../../api/api';
import { updateReleaseDialog, useOverview } from '../../utils/store';
import { Button } from '../button';
import { Locks, LocksWhite } from '../../../images';
import { Chip } from '../chip';

export type ReleaseDialogProps = {
    className?: string;
    app: string;
    version: number;
    release: Release;
    envs: Environment[];
};

const setClosed = () => {
    updateReleaseDialog('', 0);
};

export type EnvSortOrder = { [index: string]: number };

// do not rename!
// these are mapped directly to css classes in chip.tsx
export enum EnvPrio {
    PROD,
    PRE_PROD,
    UPSTREAM,
    OTHER,
}

export type EnvPrioMap = { [key: string]: EnvPrio };

/**
 * We have an arbitrary number of environments.
 * This function groups them. There are 4 groups:
 * prod: anything at the root of the tree
 * pre-prod: anything directly before prod.
 * upstream: anything with upstream.latest=true
 * other: anything else
 */
export const calculateEnvironmentPriorities = (envs: Environment[]): EnvPrioMap => {
    const distances: EnvSortOrder = calculateDistanceToUpstream(envs);
    const result: EnvPrioMap = {};
    let maxDistance = 0;
    // first, find the maximum...
    envs.forEach((env: Environment) => {
        maxDistance = Math.max(maxDistance, distances[env.name]);
    });
    // now assign each environment a prio:
    envs.forEach((env: Environment) => {
        if (distances[env.name] === maxDistance) {
            result[env.name] = EnvPrio.PROD;
        } else if (distances[env.name] === maxDistance - 1) {
            result[env.name] = EnvPrio.PRE_PROD;
        } else if (distances[env.name] === 0) {
            result[env.name] = EnvPrio.UPSTREAM;
        } else {
            result[env.name] = EnvPrio.OTHER;
        }
    });
    return result;
};

export const sortEnvironmentsByUpstream = (envs: Environment[]): Environment[] => {
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
    return sortedEnvs;
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

export const EnvironmentList: React.FC<{ envs: Environment[]; release: Release; app: string; className?: string }> = ({
    envs,
    release,
    app,
    className,
}) => {
    const allEnvs: Environment[] = useOverview((x) => Object.values(x.environments));
    const envPrioMap: EnvPrioMap = calculateEnvironmentPriorities(allEnvs);
    return (
        <ul className={classNames('release-env-list', className)}>
            {sortEnvironmentsByUpstream(envs).map((env) => (
                <li key={env.name} className={classNames('env-card', className)}>
                    <div className="env-card-header">
                        <Chip
                            className={'release-environment'}
                            label={env.name}
                            key={env.name}
                            priority={envPrioMap[env.name]}
                        />
                        <div className={classNames('_env-card-label', className)}>
                            {Object.values(env.locks).length !== 0 ? (
                                <div className={classNames('env-card-env-locks', className)}>
                                    {Object.values(env.locks).map((lock) => (
                                        <Tooltip
                                            className="env-card-env-lock"
                                            key={lock.lockId}
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
                                                    icon={<LocksWhite className="env-card-env-lock-icon" />}
                                                    className={'button-lock'}
                                                />
                                            </div>
                                        </Tooltip>
                                    ))}
                                </div>
                            ) : (
                                <></>
                            )}
                        </div>
                        <div className={classNames('env-card-app-locks')}>
                            {Object.values(env.applications)
                                .filter((application) => application.name === app)
                                .map((app) => app.locks)
                                .map((locks) =>
                                    Object.values(locks).map((lock) => (
                                        <Tooltip
                                            key={lock.lockId}
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
                                                    icon={<Locks className="env-card-app-lock" />}
                                                    className={'button-lock'}
                                                />
                                            </div>
                                        </Tooltip>
                                    ))
                                )}
                        </div>
                    </div>
                    <div className={classNames('env-card-data', className)}>
                        {release.version === env.applications[app].version
                            ? release.sourceCommitId + ':' + release.sourceMessage
                            : env.name + ' is deployed to version ' + env.applications[app].version}
                    </div>
                    <div className="env-card-buttons">
                        <Button className="env-card-add-lock-btn" label="Add lock" icon={<Locks className="icon" />} />
                        <Button className="env-card-deploy-btn" label="Deploy" />
                    </div>
                </li>
            ))}
        </ul>
    );
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const { app, className, release, envs } = props;
    const dialog =
        app !== '' ? (
            <div>
                <Dialog
                    className={classNames('release-dialog', className)}
                    fullWidth={true}
                    maxWidth="md"
                    open={app !== ''}
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
                                {release?.sourceAuthor ? 'Author: ' + release?.sourceAuthor : ''}
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
                    <EnvironmentList app={app} envs={envs} className={className} release={release} />
                </Dialog>
            </div>
        ) : (
            ''
        );

    return <div>{dialog}</div>;
};
