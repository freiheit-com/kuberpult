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
import {
    addAction,
    useCloseReleaseDialog,
    useEnvironmentGroups,
    useRelease,
    useReleaseOptional,
    useTeamFromApplication,
} from '../../utils/store';
import { Button } from '../button';
import { Close, Locks } from '../../../images';
import { EnvironmentChip } from '../chip/EnvironmentGroupChip';
import { getFormattedReleaseDate } from '../ReleaseCard/ReleaseCard';

export type ReleaseDialogProps = {
    className?: string;
    app: string;
    version: number;
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

export type EnvironmentListItemProps = {
    env: Environment;
    app: string;
    release: Release;
    queuedVersion: number;
    className?: string;
};

export const EnvironmentListItem: React.FC<EnvironmentListItemProps> = ({
    env,
    app,
    release,
    queuedVersion,
    className,
}) => {
    const deploy = useCallback(() => {
        if (release.version) {
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
        }
    }, [app, env.name, release.version]);
    const createAppLock = useCallback(() => {
        const randBase36 = (): string => Math.random().toString(36).substring(7);
        const randomLockId = (): string => 'ui-v2-' + randBase36();
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
                className={classNames('env-card-data env-card-data-queue', className)}
                title={
                    'An attempt was made to deploy version ' +
                    queuedVersion +
                    ' either by a release train, or when a new version was created. However, there was a lock present at the time, so kuberpult did not deploy this version. '
                }>
                Version {queuedVersion} was not deployed, because of a lock.
            </div>
        );
    const otherRelease = useReleaseOptional(app, env);
    const application = env.applications[app];
    const getCommitString = (): string => {
        if (!application) {
            return `"${app}" has no version deployed on "${env.name}"`;
        }
        if (release.undeployVersion) {
            return 'Undeploy Version';
        }
        if (release.version === application.version) {
            return release.sourceCommitId + ': ' + release.sourceMessage;
        }
        if (otherRelease?.undeployVersion) {
            return 'Undeploy Version';
        }
        return otherRelease?.sourceCommitId + ': ' + otherRelease?.sourceMessage;
    };
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
            <div className="content-area">
                <div className="content-left">
                    <div
                        className={classNames('env-card-data', className)}
                        title={
                            'Shows the version that is currently deployed on ' +
                            env.name +
                            '. ' +
                            (release.undeployVersion ? undeployTooltipExplanation : '')
                        }>
                        {getCommitString()}
                    </div>
                    {queueInfo}
                </div>
                <div className="content-right">
                    <div className="env-card-buttons">
                        <Button
                            className="env-card-add-lock-btn"
                            label="Add lock"
                            onClick={createAppLock}
                            icon={<Locks className="icon" />}
                        />
                        <Button
                            disabled={application && application.version === release.version}
                            className={classNames('env-card-deploy-btn', 'mdc-button--unelevated')}
                            onClick={deploy}
                            label="Deploy"
                        />
                    </div>
                </div>
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
    const allEnvGroups: EnvironmentGroup[] = useEnvironmentGroups();
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

export const undeployTooltipExplanation =
    'This is the "undeploy" version. It is essentially an empty manifest. Deploying this means removing all kubernetes entities like deployments from the given environment. You must deploy this to all environments before kuberpult allows to delete the app entirely.';

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const { app, className, version } = props;
    const release = useRelease(app, version);
    const team = useTeamFromApplication(app);
    const closeReleaseDialog = useCloseReleaseDialog();
    const undeployVersionTitle = release.undeployVersion
        ? undeployTooltipExplanation
        : 'Commit Hash of the source repository.';
    const dialog =
        app !== '' ? (
            <div>
                <Dialog
                    className={classNames('release-dialog', className)}
                    fullWidth={true}
                    maxWidth="md"
                    open={app !== ''}
                    onClose={closeReleaseDialog}>
                    <div className={classNames('release-dialog-app-bar', className)}>
                        <div className={classNames('release-dialog-app-bar-data')}>
                            <div className={classNames('release-dialog-message', className)}>
                                <span className={classNames('release-dialog-commitMessage', className)}>
                                    {release?.sourceMessage}
                                </span>
                            </div>
                            <div className={classNames('release-dialog-createdAt', className)}>
                                {!!release?.createdAt && getFormattedReleaseDate(release.createdAt)}
                            </div>
                            <div className={classNames('release-dialog-author', className)}>
                                {release?.sourceAuthor ? 'Author: ' + release?.sourceAuthor : ''}
                            </div>
                            <div className={classNames('release-dialog-app', className)}>
                                {`App: ${app} ` + (team ? ` | Team: ${team}` : '')}
                            </div>
                        </div>
                        <span className={classNames('release-dialog-commitId', className)} title={undeployVersionTitle}>
                            {release.undeployVersion ? 'Undeploy Version' : release?.sourceCommitId}
                        </span>
                        <Button
                            onClick={closeReleaseDialog}
                            className={classNames('release-dialog-close', className)}
                            icon={<Close />}
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
