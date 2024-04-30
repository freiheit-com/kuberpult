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
import { Button } from '../button';
import { Delete } from '../../../images';
import { addAction, DisplayLock } from '../../utils/store';
import classNames from 'classnames';
import { useCallback } from 'react';
import { FormattedDate } from '../FormattedDate/FormattedDate';

const millisecondsPerDay = 1000 * 60 * 60 * 24;
// lock is outdated if it's more than two days old
export const isOutdated = (dateAdded: Date | undefined): boolean =>
    dateAdded ? (Date.now().valueOf() - dateAdded.valueOf()) / millisecondsPerDay > 2 : true;

export const LockDisplay: React.FC<{ lock: DisplayLock }> = (props) => {
    const { lock } = props;
    const allClassNames = classNames('lock-display-info', {
        'date-display--outdated': isOutdated(lock.date),
        'date-display--normal': !isOutdated(lock.date),
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
    }, [lock.application, lock.environment, lock.lockId]);
    return (
        <div className="lock-display">
            <div className="lock-display__table">
                <div className="lock-display-table">
                    {!!lock.date && <FormattedDate createdAt={lock.date} className={allClassNames} />}
                    <div className="lock-display-info">{lock.environment}</div>
                    {!!lock.application && <div className="lock-display-info">{lock.application}</div>}
                    <div className="lock-display-info">{lock.lockId}</div>
                    <div className="lock-display-info">{lock.message}</div>
                    <div className="lock-display-info">{lock.authorName}</div>
                    <div className="lock-display-info">{lock.authorEmail}</div>
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
