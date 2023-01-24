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

import { sortEnvLocksFromIDs } from '../../utils/store';
import { EnvLockDisplay } from '../LockDisplay/EnvLockDisplay';
import * as React from 'react';
import { Button } from '../button';
import { SortAscending, SortDescending } from '../../../images';
import { useCallback } from 'react';

export const EnvLocksTable: React.FC<{
    headerTitle: string;
    columnHeaders: string[];
    lockIDs: string[];
}> = (props) => {
    const { headerTitle, columnHeaders, lockIDs } = props;

    const [sort, setSort] = React.useState<'ascending' | 'descending'>('ascending');

    const sortOnClick = useCallback(() => {
        if (sort === 'ascending') {
            setSort('descending');
        } else {
            setSort('ascending');
        }
        sortEnvLocksFromIDs(lockIDs, sort);
    }, [lockIDs, sort]);

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
                                            {columnHeader === 'Date' && sort === 'ascending' && (
                                                <Button
                                                    className={'mdc-data-indicator-sort-button'}
                                                    onClick={sortOnClick}
                                                    icon={<SortAscending />}
                                                />
                                            )}
                                            {columnHeader === 'Date' && sort === 'descending' && (
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
                                {lockIDs.map((lockId) => (
                                    <EnvLockDisplay key={lockId} lockID={lockId} />
                                ))}
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    );
};
