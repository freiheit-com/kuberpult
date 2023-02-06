import { Button } from '../button';
import { Delete } from '../../../images';
import { DisplayLock } from '../../utils/store';
import classNames from 'classnames';

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

export const LockDisplay: React.FC<{ lock: DisplayLock }> = (props) => {
    const { lock } = props;

    const allClassNames = classNames('lock-display-info', {
        'date-display--outdated': isOutdated(lock.date),
        'date-display--normal': !isOutdated(lock.date),
    });

    return (
        <div className="lock-display">
            <div className="lock-display__table">
                <div className="lock-display-table">
                    <div className={allClassNames}>{daysToString(calcLockAge(lock.date))}</div>
                    <div className="lock-display-info">{lock.environment}</div>
                    {!!lock.application && <div className="lock-display-info">{lock.application}</div>}
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
