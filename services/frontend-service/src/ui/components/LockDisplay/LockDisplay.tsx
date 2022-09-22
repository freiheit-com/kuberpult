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
import { Button } from '../button';
import { Delete } from '../../../images';
import { DisplayLock } from '../../../api/api';

export const LockDisplay: React.FC<{ lock: DisplayLock }> = (props) => {
    const { lock } = props;
    return (
        <div className="lock-display">
            <div className="-lock-display__table">
                <div className="lock-display-table">
                    <div
                        className={
                            'lock-display-info date-display--' + (isOutdated(lock.date) ? 'outdated' : 'normal')
                        }>
                        {daysToString(calcLockAge(lock.date))}
                    </div>
                    <div className="lock-display-info">{lock.environment}</div>
                    {lock.application !== undefined && <div className="lock-display-info">{lock.application}</div>}
                    <div className="lock-display-info">{lock.lockId}</div>
                    <div className="lock-display-info">{lock.message}</div>
                    <div className="lock-display-info">{lock.authorName}</div>
                    <div className="lock-display-info">{lock.authorEmail}</div>
                    <Button className="lock-display-info lock-action service-action--delete" icon={<Delete />} />
                </div>
            </div>
        </div>
    );
};

const daysToString = (days: number) => {
    if (days === -1) return '';
    if (days === 0) return '< 1 day ago';
    if (days === 1) return '1 day ago';
    return `${days} days ago`;
};

const calcLockAge = (time: Date | string | undefined): number => {
    if (time !== undefined && typeof time !== 'string') {
        const curTime = new Date().getTime();
        const diffTime = curTime.valueOf() - time.valueOf();
        const msPerDay = 1000 * 60 * 60 * 24;
        return Math.floor(diffTime / msPerDay);
    }
    return -1;
};

const isOutdated = (dateAdded: Date | string | undefined): boolean => {
    if (dateAdded !== undefined && typeof dateAdded !== 'string') {
        return calcLockAge(dateAdded) > 2;
    }
    return false;
};
