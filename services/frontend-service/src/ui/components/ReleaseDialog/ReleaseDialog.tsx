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

Copyright freiheit.com*/
import classNames from 'classnames';
import React, { ReactElement, useCallback } from 'react';
import { Deployment, Environment, EnvironmentGroup, Lock, LockBehavior, Release } from '../../../api/api';
import {
    addAction,
    getPriorityClassName,
    showSnackbarWarn,
    useActions,
    useAppDetailsForApp,
    useApplications,
    useCloseReleaseDialog,
    useCurrentlyDeployedAtGroup,
    useEnvironmentGroups,
    useReleaseDifference,
    useReleaseOptional,
    useRolloutStatus,
    useTeamFromApplication,
    useTeamLocks,
} from '../../utils/store';
import { Button } from '../button';
import { Close, Locks, SortAscending, SortDescending } from '../../../images';
import { EnvironmentChip } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import {
    ArgoAppLink,
    ArgoTeamLink,
    DisplayManifestLink,
    DisplaySourceLink,
    DisplayCommitHistoryLink,
} from '../../utils/Links';
import { ReleaseVersion } from '../ReleaseVersion/ReleaseVersion';
import { PlainDialog } from '../dialog/ConfirmationDialog';
import { ExpandButton } from '../button/ExpandButton';
import { RolloutStatusDescription } from '../RolloutStatusDescription/RolloutStatusDescription';

export type ReleaseDialogProps = {
    className?: string;
    app: string;
    version: number;
};

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
        <div
            title={'App Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}
            onClick={deleteAppLock}>
            <Button icon={<Locks className="env-card-app-lock" />} className={'button-lock'} highlightEffect={false} />
        </div>
    );
};

export const TeamLock: React.FC<{
    env: Environment;
    team: string;
    lock: Lock;
}> = ({ env, team, lock }) => {
    const deleteTeamLock = useCallback(() => {
        addAction({
            action: {
                $case: 'deleteEnvironmentTeamLock',
                deleteEnvironmentTeamLock: { environment: env.name, team: team, lockId: lock.lockId },
            },
        });
    }, [team, env.name, lock.lockId]);
    return (
        <div
            title={'Team Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}
            onClick={deleteTeamLock}>
            <Button icon={<Locks className="env-card-app-lock" />} className={'button-lock'} highlightEffect={false} />
        </div>
    );
};

export type EnvironmentListItemProps = {
    env: Environment;
    envGroup: EnvironmentGroup;
    app: string;
    release: Release;
    queuedVersion: number;
    className?: string;
    team?: string;
};

type CommitIdProps = {
    deployment: Deployment | undefined;
    app: string;
    env: Environment;
    otherRelease?: Release;
};

const DeployedVersion: React.FC<CommitIdProps> = ({ deployment, app, env, otherRelease }): ReactElement => {
    if (!deployment || !otherRelease) {
        return (
            <span>
                "{app}" has no version deployed on "{env.name}"
            </span>
        );
    }
    const firstLine = otherRelease.sourceMessage.split('\n')[0];
    return (
        <span>
            <ReleaseVersion release={otherRelease} />
            {firstLine}
        </span>
    );
};

export const EnvironmentListItem: React.FC<EnvironmentListItemProps> = ({
    env,
    envGroup,
    app,
    release,
    queuedVersion,
    className,
    team,
}) => {
    const createAppLock = useCallback(() => {
        addAction({
            action: {
                $case: 'createEnvironmentApplicationLock',
                createEnvironmentApplicationLock: {
                    environment: env.name,
                    application: app,
                    lockId: '',
                    message: '',
                    ciLink: '',
                },
            },
        });
    }, [app, env.name]);
    const deployAndLockClick = useCallback(
        (shouldLockToo: boolean) => {
            if (!release.environments.includes(env.name)) {
                showSnackbarWarn(`Environments skipped: ${env.name}`);
                return;
            }
            if (release.version) {
                addAction({
                    action: {
                        $case: 'deploy',
                        deploy: {
                            environment: env.name,
                            application: app,
                            version: release.version,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.IGNORE,
                        },
                    },
                });
                if (shouldLockToo) {
                    createAppLock();
                }
            }
        },
        [release.version, release.environments, app, env.name, createAppLock]
    );

    const queueInfo =
        queuedVersion === 0 ? null : (
            <div
                className={classNames('env-card-data env-card-data-queue')}
                title={
                    'An attempt was made to deploy version ' +
                    queuedVersion +
                    ' either by a release train, or when a new version was created. However, there was a lock present at the time, so kuberpult did not deploy this version. '
                }>
                Version {queuedVersion} was not deployed, because of a lock.
            </div>
        );
    const otherRelease = useReleaseOptional(app, env);
    const appDetails = useAppDetailsForApp(app);
    const deployment = appDetails.details?.deployments[env.name];

    const getDeploymentMetadata = (): [String, JSX.Element] => {
        if (!deployment) {
            return ['', <></>];
        }
        if (deployment.deploymentMetaData === null) {
            return ['', <></>];
        }
        const deployedBy = deployment.deploymentMetaData?.deployAuthor ?? 'unknown';
        const deployedUNIX = deployment.deploymentMetaData?.deployTime ?? '';
        if (deployedUNIX === '') {
            return ['Deployed by &nbsp;' + deployedBy, <></>];
        }
        const deployedDate = new Date(deployedUNIX);
        const returnString = 'Deployed by ' + deployedBy + ' ';
        const time = <FormattedDate createdAt={deployedDate} className={classNames('release-dialog-createdAt', '')} />;

        return [returnString, time];
    };
    const appRolloutStatus = useRolloutStatus((getter) => getter.getAppStatus(app, deployment?.version, env.name));
    const apps = useApplications().filter((application) => application.name === app);
    const teamLocks = useTeamLocks(apps).filter((lock) => lock.environment === env.name);
    const appEnvLocks = appDetails?.details?.appLocks?.[env.name]?.locks ?? [];

    const allowDeployment: boolean = ((): boolean => {
        if (release.isPrepublish) {
            return false;
        }
        if (!otherRelease) {
            return true;
        }
        return otherRelease.version !== release.version;
    })();

    const releaseDifference = useReleaseDifference(release.version, app, env.name);
    const getReleaseDiffContent = (): JSX.Element => {
        if (!otherRelease || !deployment) {
            return <div></div>;
        }
        if (releaseDifference > 0) {
            return (
                <div>
                    <span className="env-card-release-diff-positive">{releaseDifference}</span> versions ahead
                </div>
            );
        } else if (releaseDifference < 0) {
            return (
                <div>
                    <span className="env-card-release-diff-negative">{releaseDifference * -1}</span> versions behind
                </div>
            );
        }
        return <div>same version</div>;
    };

    const actions = useActions();
    const alreadyPlanned = actions.some(
        (action) =>
            action.action?.$case === 'deploy' &&
            action.action.deploy.application === app &&
            action.action.deploy.environment === env.name
    );
    const defaultLabel =
        releaseDifference < 0 ? 'Update & Lock' : releaseDifference === 0 ? 'Deploy & Lock' : 'Rollback & Lock';

    return (
        <li key={env.name} className={classNames('env-card')}>
            <div className="env-card-header">
                <EnvironmentChip
                    env={env}
                    app={app}
                    envGroup={envGroup}
                    className={'release-environment'}
                    key={env.name}
                    groupNameOverride={undefined}
                    numberEnvsDeployed={undefined}
                    numberEnvsInGroup={undefined}
                    useEnvColor={false}
                />
                <div className={classNames('env-card-locks')}>
                    {appEnvLocks.length > 0 && (
                        <div className={classNames('env-card-app-locks')}>
                            App:
                            {Object.values(appEnvLocks).map((lock) => (
                                <AppLock key={lock.lockId} env={env} app={app} lock={lock} />
                            ))}
                        </div>
                    )}
                    {teamLocks.length > 0 && (
                        <div className={classNames('env-card-app-locks')}>
                            Team:
                            {Object.values(teamLocks).map((lock) => (
                                <TeamLock key={lock.lockId} env={env} team={team || ''} lock={lock} />
                            ))}
                        </div>
                    )}
                    {appRolloutStatus !== undefined && <RolloutStatusDescription status={appRolloutStatus} />}
                </div>
            </div>
            <div className="content-area">
                <div className="content-left">
                    <div
                        className={classNames('env-card-data')}
                        title={
                            'Shows the version that is currently deployed on ' +
                            env.name +
                            '. ' +
                            (release.undeployVersion ? undeployTooltipExplanation : '')
                        }>
                        <DeployedVersion app={app} env={env} deployment={deployment} otherRelease={otherRelease} />
                    </div>
                    {queueInfo}
                    <div className={classNames('env-card-data')}>
                        {getDeploymentMetadata().flatMap((metadata, i) => (
                            <div key={i}>{metadata}&nbsp;</div>
                        ))}
                    </div>
                    <div className={classNames('env-card-data')}>{getReleaseDiffContent()}</div>
                </div>
                <div className="content-right">
                    <div className="env-card-buttons">
                        <div
                            title={
                                'When doing manual deployments, it is usually best to also lock the app. If you omit the lock, an automatic release train or another person may deploy an unintended version. If you do not want a lock, click the arrow.'
                            }>
                            <ExpandButton
                                onClickSubmit={deployAndLockClick}
                                onClickLock={createAppLock}
                                defaultButtonLabel={alreadyPlanned ? `Cancel ${defaultLabel}` : defaultLabel}
                                releaseDifference={releaseDifference}
                                disabled={!allowDeployment}
                                alreadyPlanned={alreadyPlanned}
                            />
                        </div>
                    </div>
                </div>
            </div>
        </li>
    );
};

export const EnvironmentList: React.FC<{
    release: Release;
    app: string;
    team: string;
    className?: string;
}> = ({ release, app, className, team }) => {
    const allEnvGroups: EnvironmentGroup[] = useEnvironmentGroups();
    return (
        <div className="release-env-group-list">
            {allEnvGroups.map((envGroup) => (
                <EnvironmentGroupLane
                    key={envGroup.environmentGroupName}
                    environmentGroup={envGroup}
                    app={app}
                    release={release}
                    team={team}
                />
            ))}
        </div>
    );
};

export const undeployTooltipExplanation =
    'This is the "undeploy" version. It is essentially an empty manifest. Deploying this means removing all kubernetes entities like deployments from the given environment. You must deploy this to all environments before kuberpult allows to delete the app entirely.';

export const ReleaseDialog: React.FC<ReleaseDialogProps> = (props) => {
    const { app, className, version } = props;
    const appDetails = useAppDetailsForApp(app);
    const team = useTeamFromApplication(app) || '';
    const closeReleaseDialog = useCloseReleaseDialog();
    if (!appDetails) {
        return null;
    }
    const release = appDetails.details?.application?.releases.find((r) => r.version === version);

    if (!release) {
        return null;
    }

    const dialog: JSX.Element | '' = (
        <PlainDialog
            open={app !== ''}
            onClose={closeReleaseDialog}
            classNames={'release-dialog'}
            disableBackground={true}
            center={true}>
            <>
                <div className={classNames('release-dialog-app-bar', className)}>
                    <div className={classNames('release-dialog-app-bar-data')}>
                        <div className={classNames('release-dialog-message', className)}>
                            <span className={classNames('release-dialog-commitMessage', className)}>
                                {release?.sourceMessage}
                            </span>
                        </div>
                        <div className="source">
                            <span>
                                {'Created '}
                                {release?.createdAt ? (
                                    <FormattedDate
                                        createdAt={release.createdAt}
                                        className={classNames('release-dialog-createdAt', className)}
                                    />
                                ) : (
                                    'at an unknown date'
                                )}
                                {' by '}
                                {release?.sourceAuthor ? release?.sourceAuthor : 'an unknown author'}{' '}
                            </span>
                            <span className="links">
                                <DisplaySourceLink commitId={release.sourceCommitId} displayString={'Source'} />
                                &nbsp;
                                <DisplayManifestLink app={app} version={release.version} displayString="Manifest" />
                                &nbsp;
                                <DisplayCommitHistoryLink
                                    commitId={release.sourceCommitId}
                                    displayString={'Commit History'}
                                />
                            </span>
                        </div>
                        <div className={classNames('release-dialog-app', className)}>
                            {'App: '}
                            <ArgoAppLink app={app} />
                            <ArgoTeamLink team={team} />
                        </div>
                    </div>
                    <Button
                        onClick={closeReleaseDialog}
                        className={classNames('release-dialog-close', className)}
                        icon={<Close />}
                        highlightEffect={false}
                    />
                </div>
                <EnvironmentList app={app} team={team} className={className} release={release} />
            </>
        </PlainDialog>
    );
    return <div>{dialog}</div>;
};

export const EnvironmentGroupLane: React.FC<{
    environmentGroup: EnvironmentGroup;
    release: Release;
    app: string;
    team: string;
}> = (props) => {
    const { environmentGroup, release, app, team } = props;
    // all envs in the same group have the same priority
    const priorityClassName = getPriorityClassName(environmentGroup);
    const [isCollapsed, setIsCollapsed] = React.useState(false);
    const [allowGroupDeployment, setAllowGroupDeployment] = React.useState(true);
    const appDetails = useAppDetailsForApp(app);
    const collapse = useCallback(() => {
        setIsCollapsed(!isCollapsed);
    }, [isCollapsed]);

    const allReleases = useCurrentlyDeployedAtGroup(app, release.version).filter(
        (releaseEnvGroup) => releaseEnvGroup.environmentGroupName === environmentGroup.environmentGroupName
    );

    const actions = useActions();
    const envsWithoutPlannedDeployments = environmentGroup.environments.filter(
        (env) =>
            !actions.some(
                (action) =>
                    action.action?.$case === 'deploy' &&
                    action.action.deploy.application === app &&
                    action.action.deploy.environment === env.name
            )
    );
    const envsWithPlannedDeployments = environmentGroup.environments.filter((env) =>
        actions.some(
            (action) =>
                action.action?.$case === 'deploy' &&
                action.action.deploy.application === app &&
                action.action.deploy.environment === env.name
        )
    );
    const alreadyPlanned = envsWithoutPlannedDeployments.length === 0;

    const createEnvGroupLock = React.useCallback(() => {
        environmentGroup.environments.forEach((environment) => {
            addAction({
                action: {
                    $case: 'createEnvironmentApplicationLock',
                    createEnvironmentApplicationLock: {
                        environment: environment.name,
                        application: app,
                        lockId: '',
                        message: '',
                        ciLink: '',
                    },
                },
            });
        });
    }, [environmentGroup, app]);
    const deployAndLockClick = React.useCallback(
        (shouldLockToo: boolean) => {
            var skippedEnvs: string[] = alreadyPlanned ? [] : envsWithPlannedDeployments.map((env) => env.name);
            const envs = alreadyPlanned ? environmentGroup.environments : envsWithoutPlannedDeployments;
            envs.forEach((environment) => {
                if (
                    allReleases &&
                    allReleases.length !== 0 &&
                    allReleases[0].environments.find((env) => env === environment)
                ) {
                    return;
                }
                if (!release.environments.includes(environment.name)) {
                    skippedEnvs.push(environment.name);
                    return;
                }
                addAction({
                    action: {
                        $case: 'deploy',
                        deploy: {
                            environment: environment.name,
                            application: app,
                            version: release.version,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.IGNORE,
                        },
                    },
                });
                if (shouldLockToo) {
                    addAction({
                        action: {
                            $case: 'createEnvironmentApplicationLock',
                            createEnvironmentApplicationLock: {
                                environment: environment.name,
                                application: app,
                                lockId: '',
                                message: '',
                                ciLink: '',
                            },
                        },
                    });
                }
            });
            if (skippedEnvs.length > 0) {
                showSnackbarWarn(`Environments skipped: ${skippedEnvs}`);
            }
        },
        [
            environmentGroup.environments,
            allReleases,
            release.environments,
            release.version,
            app,
            alreadyPlanned,
            envsWithoutPlannedDeployments,
            envsWithPlannedDeployments,
        ]
    );

    React.useEffect(() => {
        //If current release is deployed on all envs of env group, we disable the groupDeploy button
        if (allReleases === undefined) {
            setAllowGroupDeployment(true);
            return;
        }

        if (allReleases.length === 0) {
            setAllowGroupDeployment(true);
        } else {
            setAllowGroupDeployment(
                JSON.stringify(allReleases[0].environments) !== JSON.stringify(environmentGroup.environments)
            );
        }
    }, [allReleases, environmentGroup]);

    return (
        <div className="release-dialog-environment-group-lane">
            <div className={'release-dialog-environment-group-lane__header-wrapper'}>
                <div className={classNames('release-dialog-environment-group-lane__header', priorityClassName)}>
                    <div className="environment-group__name" title={'Name of the environment group'}>
                        {environmentGroup.environmentGroupName}
                    </div>
                    {isCollapsed ? (
                        <div className={'collapse-dropdown-arrow-container'}>
                            <Button onClick={collapse} icon={<SortDescending />} highlightEffect={false} />
                        </div>
                    ) : (
                        <div className={'collapse-dropdown-arrow-container'}>
                            <Button onClick={collapse} icon={<SortAscending />} highlightEffect={false} />
                        </div>
                    )}
                </div>
                <div className="env-group-card-buttons">
                    <div
                        className={'env-group-expand-button'}
                        title={
                            'When doing manual deployments, it is usually best to also lock the app. If you omit the lock, an automatic release train or another person may deploy an unintended version. If you do not want a lock, click the arrow.'
                        }>
                        <ExpandButton
                            onClickSubmit={deployAndLockClick}
                            onClickLock={createEnvGroupLock}
                            defaultButtonLabel={alreadyPlanned ? 'Cancel Deploy & Lock' : 'Deploy & Lock'}
                            disabled={!allowGroupDeployment}
                            releaseDifference={0}
                            alreadyPlanned={alreadyPlanned}
                        />
                    </div>
                </div>
            </div>
            {isCollapsed ? (
                <div className={'release-dialog-environment-group-lane__body__collapsed'}></div>
            ) : (
                <div className="release-dialog-environment-group-lane__body">
                    {environmentGroup.environments.map((env) => (
                        <EnvironmentListItem
                            key={env.name}
                            env={env}
                            envGroup={environmentGroup}
                            app={app}
                            release={release}
                            team={team}
                            className={priorityClassName}
                            queuedVersion={
                                appDetails.details?.deployments[env.name]
                                    ? appDetails.details?.deployments[env.name].queuedVersion
                                    : 0
                            }
                        />
                    ))}
                </div>
            )}

            {/*I am just here so that we can avoid margin collapsing */}
            <div className={'release-dialog-environment-group-lane__footer'} />
        </div>
    );
};
