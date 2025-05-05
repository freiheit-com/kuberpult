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
import { addAction, DisplayLock } from '../../utils/store';
import classNames from 'classnames';
import { useCallback } from 'react';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { Link } from 'react-router-dom';

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
    const deleteLock = useCallback(() => {
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
    }, [lock.application, lock.environment, lock.lockId, lock.team]);

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
        </div>
    );
};

const GetTargetFutureDate = (current: Date | undefined, increment: string): Date | undefined => {
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
