/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/
import { Dialog, Tooltip } from '@material-ui/core';
import classNames from 'classnames';
import React, { useCallback } from 'react';
import { Environment, EnvironmentGroup, Lock, LockBehavior, Release } from '../../../api/api';
import { addAction, updateReleaseDialog, useOverview } from '../../utils/store';
import { Button } from '../button';
import { Locks } from '../../../images';
import { EnvironmentChip } from '../chip/EnvironmentGroupChip';

export type ReleaseDialogProps = {
    className?: string;
    app: string;
    version: number;
    release: Release;
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

export const AppLock: React.FC<{
    env: Environment;
    app: string;
    lock: Lock;
}> = ({ env, app, lock }) => {
    const deleteAppLock = useCallback(() => {
        addAction({
            action: {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: { environment: env.name, application: app, lockId: lock.lockId },
            },
        });
    }, [app, env.name, lock.lockId]);
    return (
        <Tooltip
            key={lock.lockId}
            arrow
            title={'Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}
            onClick={deleteAppLock}>
            <div>
                <Button icon={<Locks className="env-card-app-lock" />} className={'button-lock'} />
            </div>
        </Tooltip>
    );
};

export const EnvironmentListItem: React.FC<{
    env: Environment;
    app: string;
    release: Release;
    queuedVersion: number;
    className?: string;
}> = ({ env, app, release, queuedVersion, className }) => {
    const deploy = useCallback(() => {
        if (release.version)
            addAction({
                action: {
                    $case: 'deploy',
                    deploy: {
                        environment: env.name,
                        application: app,
                        version: release.version,
                        ignoreAllLocks: false,
                        lockBehavior: LockBehavior.Ignore,
                    },
                },
            });
    }, [app, env.name, release.version]);
    const createAppLock = useCallback(() => {
        const randBase36 = () => Math.random().toString(36).substring(7);
        const randomLockId = () => 'ui-v2-' + randBase36();
        addAction({
            action: {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: {
                    environment: env.name,
                    application: app,
                    lockId: randomLockId(),
                    message: '',
                },
            },
        });
    }, [app, env.name]);
    const queueInfo =
        queuedVersion === 0 ? null : (
            <div
                className={classNames('env-card-data-queue', className)}
                title={
                    'An attempt was made to deploy version ' +
                    queuedVersion +
                    ' either by a release train, or when a new version was created. However, there was a lock present at the time, so kuberpult did not deploy this version. '
                }>
                Version {queuedVersion} was not deployed, because of a lock.
            </div>
        );
    return (
        <li key={env.name} className={classNames('env-card', className)}>
            <div className="env-card-header">
                <EnvironmentChip
                    env={env}
                    className={'release-environment'}
                    key={env.name}
                    groupNameOverride={undefined}
                    numberEnvsDeployed={undefined}
                    numberEnvsInGroup={undefined}
                    withEnvLocks={true}
                />
                <div className={classNames('env-card-app-locks')}>
                    {Object.values(env.applications)
                        .filter((application) => application.name === app)
                        .map((app) => app.locks)
                        .map((locks) =>
                            Object.values(locks).map((lock) => (
                                <AppLock key={lock.lockId} env={env} app={app} lock={lock} />
                            ))
                        )}
                </div>
            </div>
            <div className={classNames('env-card-data', className)}>
                {env.applications[app]
                    ? release.version === env.applications[app].version
                        ? release.sourceCommitId + ':' + release.sourceMessage
                        : env.name + ' is deployed to version ' + env.applications[app].version
                    : `"${app}" has no version deployed on "${env.name}"`}
            </div>
            {queueInfo}
            <div className="env-card-buttons">
                <Button
                    className="env-card-add-lock-btn"
                    label="Add lock"
                    onClick={createAppLock}
                    icon={<Locks className="icon" />}
                />
                <Button
                    disabled={!release.version}
                    className={classNames('env-card-deploy-btn', { 'btn-disabled': !release.version })}
                    onClick={deploy}
                    label="Deploy"
                />
            </div>
        </li>
    );
};

export const EnvironmentList: React.FC<{
    release: Release;
    app: string;
    version: number;
    className?: string;
}> = ({ release, app, version, className }) => {
    const allEnvGroups: EnvironmentGroup[] = useOverview((x) => Object.values(x.environmentGroups));
    return (
        <div className="release-env-group-list">
            {allEnvGroups.map((envGroup) => (
                <ul className={classNames('release-env-list', className)} key={envGroup.environmentGroupName}>
                    {envGroup.environments.map((env) => (
                        <EnvironmentListItem
                            key={env.name}
                            env={env}
                            app={app}
                            release={release}
                            className={className}
                            queuedVersion={env.applications[app] ? env.applications[app].queuedVersion : 0}
                        />
                    ))}
                </ul>
            ))}
        </div>
    );
};

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const { app, className, release, version } = props;
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
                    <EnvironmentList
                        app={app}
                        className={className}
                        release={release}
                        version={version}
                        // deployedAtGroup={deployedAtGroup}
                    />
                </Dialog>
            </div>
        ) : (
            ''
        );

    return <div>{dialog}</div>;
};
