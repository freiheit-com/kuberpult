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

import { DisplayLock } from '../../../api/api';
import { LockDisplay } from '../LockDisplay/LockDisplay';

export const LocksTable: React.FC<{
    headerTitle: string;
    columnHeaders: string[];
    locks: DisplayLock[];
}> = (props) => {
    const { headerTitle, columnHeaders, locks } = props;
    const sortedLocks = sortLocks(locks);
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
                                        <div className="mdc-data-indicator-field">{columnHeader}</div>
                                    ))}
                                </div>
                            </th>
                        </tr>
                    </thead>
                    <tbody className="mdc-data-table__content">
                        {sortedLocks.map((lock) => (
                            <LockDisplay lock={lock} />
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

const sortLocks = (displayLocks: DisplayLock[]) =>
    displayLocks.sort((a: DisplayLock, b: DisplayLock) => {
        if (a.date === b.date) {
            if (a.environment === b.environment) {
                if (a.application !== undefined && b.application !== undefined) {
                    if (a.application === b.application) {
                        return a.lockId < b.lockId ? -1 : a.lockId === b.lockId ? 0 : 1;
                    }
                    return a.application < b.application ? -1 : 1;
                }
                return a.lockId < b.lockId ? -1 : a.lockId === b.lockId ? 0 : 1;
            }
            return a.environment < b.environment ? -1 : 1;
        }
        return a.date > b.date ? -1 : 1;
    });
