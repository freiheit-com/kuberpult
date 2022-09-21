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

export const EnvironmentLockDisplay: React.FC<{ lock: (string | Date | undefined)[] }> = (props) => {
    const { lock } = props;
    return (
        <div className="env-lock-display">
            <div className="env-lock-display__table">
                <div className="env-lock-display-table">
                    <div
                        className={
                            'env-lock-display-info date-display--' + (isOutdated(lock[0]) ? 'outdated' : 'normal')
                        }>
                        {daysToString(calcLockAge(lock[0]))}
                    </div>
                    <div className="env-lock-display-info">{lock[1]?.toString()}</div>
                    <div className="env-lock-display-info">{lock[2]?.toString()}</div>
                    <div className="env-lock-display-info">{lock[3]?.toString()}</div>
                    <div className="env-lock-display-info">{lock[4]?.toString()}</div>
                    <div className="env-lock-display-info">{lock[5]?.toString()}</div>
                    <div className="env-lock-display-info">
                        <Button className="lock-action service-action--delete" icon={<Delete />} />
                    </div>
                </div>
            </div>
        </div>
    );
};

// const getFullDate = (time: number) => {
//     // use -1 to sort the dates with the newest on top
//     time *= -1;
//     if (time === -1) return '';
//     const d = new Date(time);
//     return d.toString();
// };
//
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
