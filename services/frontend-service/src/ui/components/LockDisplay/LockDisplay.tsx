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
import { Button } from '../button';
import { Delete } from '../../../images';
import { addAction, DisplayLock, useAppDetailsForApp } from '../../utils/store';
import classNames from 'classnames';
import { useCallback, useState } from 'react';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { Link } from 'react-router-dom';
import { LockBehavior } from '../../../api/api';
import { PlainDialog } from '../dialog/ConfirmationDialog';

const millisecondsPerDay = 1000 * 60 * 60 * 24;
// lock is outdated if it's more than two days old
export const isOutdated = (dateAdded: Date | undefined): boolean =>
    dateAdded ? (Date.now().valueOf() - dateAdded.valueOf()) / millisecondsPerDay > 2 : true;

export const isOutdatedLifetime = (lifetime: Date | undefined): boolean =>
    lifetime ? Date.now().valueOf() - lifetime.valueOf() > 0 : true;

export const LockDisplay: React.FC<{ lock: DisplayLock }> = (props) => {
    const { lock } = props;
    const targetLifetimeDate = GetTargetFutureDate(lock.date, lock.suggestedLifetime);
    const allClassNames = classNames('lock-display-info', {
        'date-display--outdated': isOutdated(lock.date),
        'date-display--normal': !isOutdated(lock.date),
    });

    const classNamesLifetime = classNames('lock-display-info', {
        'date-display--outdated': isOutdatedLifetime(targetLifetimeDate),
        'date-display--normal': !isOutdatedLifetime(targetLifetimeDate),
    });
    const [showDeleteManifestLockDialog, setShowDeleteManifestLockDialog] = useState(false);
    const appDetails = useAppDetailsForApp(lock.application || '');
    const deployedVersion = lock.application ? appDetails.details?.deployments[lock.environment] : undefined;

    const addDeleteManifestLockAction = useCallback(() => {
        if (!lock.application) {
            throw new Error('manifest lock ' + lock.lockId + ' is missing the application');
        }
        addAction({
            action: {
                $case: 'deleteManifestLock',
                deleteManifestLock: {
                    app: lock.application,
                    env: lock.environment,
                },
            },
        });
    }, [lock.application, lock.environment, lock.lockId]);

    const deleteLock = useCallback(() => {
        if (lock.isManifestLock && lock.application) {
            setShowDeleteManifestLockDialog(true);
            return;
        }
        if (lock.application) {
            addAction({
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        environment: lock.environment,
                        lockId: lock.lockId,
                        application: lock.application,
                    },
                },
            });
        } else if (lock.team) {
            addAction({
                action: {
                    $case: 'deleteEnvironmentTeamLock',
                    deleteEnvironmentTeamLock: {
                        environment: lock.environment,
                        lockId: lock.lockId,
                        team: lock.team,
                    },
                },
            });
        } else {
            addAction({
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: lock.environment,
                        lockId: lock.lockId,
                    },
                },
            });
        }
    }, [lock.application, lock.environment, lock.lockId, lock.team, lock.isManifestLock]);

    const onClose = useCallback(() => {
        setShowDeleteManifestLockDialog(false);
    }, []);
    const onClickRemoveLockOnly = useCallback(() => {
        setShowDeleteManifestLockDialog(false);
        addDeleteManifestLockAction();
    }, [addDeleteManifestLockAction]);
    const onClickRemoveLockAndRedeploy = useCallback(() => {
        if (!lock.application) {
            throw new Error(
                'cannot remove lock and redeploy: manifest lock ' + lock.lockId + ' is missing the application'
            );
        }
        if (!deployedVersion) {
            throw new Error('cannot remove lock and redeploy: no deployment found');
        }
        setShowDeleteManifestLockDialog(false);
        addDeleteManifestLockAction();
        addAction({
            action: {
                $case: 'deploy',
                deploy: {
                    environment: lock.environment,
                    application: lock.application,
                    version: deployedVersion.version,
                    revision: deployedVersion.revision ?? 0,
                    ignoreAllLocks: false,
                    lockBehavior: LockBehavior.IGNORE,
                },
            },
        });
    }, [addDeleteManifestLockAction, deployedVersion, lock.environment, lock.application, lock.lockId]);

    return (
        <div className="lock-display">
            <div className="lock-display__table">
                <div className="lock-display-table">
                    {!!lock.date && <FormattedDate createdAt={lock.date} className={allClassNames} />}
                    <div className="lock-display-info">{lock.environment}</div>
                    {!!lock.application && <div className="lock-display-info">{lock.application}</div>}
                    {!!lock.team && <div className="lock-display-info">{lock.team}</div>}
                    <div className="lock-display-info">{lock.lockId}</div>
                    <div className="lock-display-info-size-limit">{lock.message}</div>
                    {lock.ciLink !== '' ? (
                        <Link
                            className="lock-display-info lock-ci-link"
                            to={lock.ciLink}
                            target="_blank"
                            rel="noopener noreferrer">
                            {lock.authorName}
                        </Link>
                    ) : (
                        <div className="lock-display-info">{lock.authorName}</div>
                    )}

                    <div className="lock-display-info">{lock.authorEmail}</div>
                    {targetLifetimeDate ? (
                        <FormattedDate
                            createdAt={targetLifetimeDate}
                            className={classNames(classNamesLifetime, 'lifetime-date')}
                        />
                    ) : (
                        <div className="lock-display-info lifetime-date">{'-'}</div>
                    )}
                    <Button
                        className="lock-display-info lock-action service-action--delete"
                        onClick={deleteLock}
                        icon={<Delete />}
                        highlightEffect={false}
                    />
                </div>
            </div>
            {lock.isManifestLock && (
                <PlainDialog
                    open={showDeleteManifestLockDialog}
                    onClose={onClose}
                    classNames="manifest-lock-dialog"
                    disableBackground={true}
                    center={true}>
                    <>
                        <div className={'manifest-lock-dialog-header'}>Remove Manifest Lock</div>
                        <div className={'manifest-lock-dialog-description'}>
                            {
                                'Removing the manifest lock will allow Kuberpult to write manifest files for this app/env again. '
                            }
                            {deployedVersion
                                ? 'You can also trigger a re-deployment of the currently deployed version to immediately restore the manifest files. ' +
                                  'This re-deployment usually has no effect unless you have manual changes in the manifest repository.'
                                : ''}
                        </div>
                        <hr />
                        <div className={'manifest-lock-dialog-footer'}>
                            <Button
                                className="mdc-button--unelevated button-cancel"
                                label="Remove lock only"
                                onClick={onClickRemoveLockOnly}
                                highlightEffect={false}
                            />
                            {deployedVersion && (
                                <Button
                                    className="mdc-button--unelevated button-confirm"
                                    label="Remove lock and re-deploy"
                                    onClick={onClickRemoveLockAndRedeploy}
                                    highlightEffect={false}
                                />
                            )}
                        </div>
                    </>
                </PlainDialog>
            )}
        </div>
    );
};

export const GetTargetFutureDate = (current: Date | undefined, increment: string): Date | undefined => {
    if (!current || increment === '') return undefined;
    const msPerMinute = 1000 * 60;
    const msPerHour = msPerMinute * 60;
    const msPerDay = msPerHour * 24;
    const msPerWeek = msPerDay * 7;

    if (increment.indexOf('w') !== -1) {
        return new Date(current.valueOf() + msPerWeek * parseInt(increment.split('w')[0]));
    } else if (increment.indexOf('d') !== -1) {
        return new Date(current.valueOf() + msPerDay * parseInt(increment.split('d')[0]));
    } else if (increment.indexOf('h') !== -1) {
        return new Date(current.valueOf() + msPerHour * parseInt(increment.split('h')[0]));
    }

    return undefined;
};
