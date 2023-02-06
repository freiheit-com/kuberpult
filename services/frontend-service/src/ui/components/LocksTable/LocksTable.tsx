import { DisplayLock, sortLocks } from '../../utils/store';
import { LockDisplay } from '../LockDisplay/LockDisplay';
import * as React from 'react';
import { Button } from '../button';
import { SortAscending, SortDescending } from '../../../images';
import { useCallback } from 'react';

export const LocksTable: React.FC<{
    headerTitle: string;
    columnHeaders: string[];
    locks: DisplayLock[];
}> = (props) => {
    const { headerTitle, columnHeaders, locks } = props;

    const [sort, setSort] = React.useState<'newestToOldest' | 'oldestToNewest'>('newestToOldest');

    const sortOnClick = useCallback(() => {
        if (sort === 'oldestToNewest') {
            setSort('newestToOldest');
        } else {
            setSort('oldestToNewest');
        }
        sortLocks(locks, sort);
    }, [locks, sort]);
    return (
        <div className="mdc-data-table">
            <div className="mdc-data-table__table-container">
                <table className="mdc-data-table__table" aria-label="Dessert calories">
                    <thead>
                        <tr className="mdc-data-table__header-row">
                            <th className="mdc-data-indicator" role="columnheader" scope="col">
                                <div className="mdc-data-header-title">{headerTitle}</div>
                            </th>
                        </tr>
                        <tr className="mdc-data-table__header-row">
                            <th
                                className="mdc-data-indicator mdc-data-indicator--subheader"
                                role="columnheader"
                                scope="col">
                                <div className="mdc-data-indicator-header">
                                    {columnHeaders.map((columnHeader) => (
                                        <div key={columnHeader} className="mdc-data-indicator-field">
                                            {columnHeader}
                                            {columnHeader === 'Date' && sort === 'oldestToNewest' && (
                                                <Button
                                                    className={'mdc-data-indicator-sort-button'}
                                                    onClick={sortOnClick}
                                                    icon={<SortAscending />}
                                                />
                                            )}
                                            {columnHeader === 'Date' && sort === 'newestToOldest' && (
                                                <Button
                                                    className={'mdc-data-indicator-sort-button'}
                                                    onClick={sortOnClick}
                                                    icon={<SortDescending />}
                                                />
                                            )}
                                        </div>
                                    ))}
                                </div>
                            </th>
                        </tr>
                    </thead>
                    <tbody className="mdc-data-table__content">
                        <tr>
                            <td>
                                {locks.map((lock) => (
                                    <LockDisplay key={lock.lockId} lock={lock} />
                                ))}
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    );
};
