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
import { useEnvironmentLock } from '../../utils/store';

export const daysToString = (days: number) => {
    if (days === -1) return '';
    if (days === 0) return '< 1 day ago';
    if (days === 1) return '1 day ago';
    return `${days} days ago`;
};

export const calcLockAge = (time: Date | string | undefined): number => {
    if (time !== undefined && typeof time !== 'string') {
        const curTime = new Date().getTime();
        const diffTime = curTime.valueOf() - time.valueOf();
        const msPerDay = 1000 * 60 * 60 * 24;
        return Math.floor(diffTime / msPerDay);
    }
    return -1;
};

export const isOutdated = (dateAdded: Date | string | undefined): boolean => calcLockAge(dateAdded) > 2;

export const LockDisplay: React.FC<{ lockId: string }> = (props) => {
    const { lockId } = props;
    const { message, environment, createdAt, createdBy } = useEnvironmentLock(lockId);

    return (
        <div className="lock-display">
            <div className="-lock-display__table">
                <div className="lock-display-table">
                    <div
                        className={
                            'lock-display-info date-display--' + (isOutdated(createdAt) ? 'outdated' : 'normal')
                        }>
                        {daysToString(calcLockAge(createdAt))}
                    </div>
                    <div className="lock-display-info">{environment}</div>
                    <div className="lock-display-info">{lockId}</div>
                    <div className="lock-display-info">{message}</div>
                    <div className="lock-display-info">{createdBy?.name}</div>
                    <div className="lock-display-info">{createdBy?.email}</div>
                    <Button className="lock-display-info lock-action service-action--delete" icon={<Delete />} />
                </div>
            </div>
        </div>
    );
};
