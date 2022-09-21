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
import { useEnvironmentLocks } from '../../utils/store';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';

export const EnvironmentLocksCard: React.FC<{}> = (props) => {
    const envlocks = useEnvironmentLocks();
    // eslint-disable-next-line no-console
    console.log(envlocks);
    return (
        <div className="mdc-env-data-table">
            <div className="mdc-env-data-table__table-container">
                <table className="mdc-env-data-table__table" aria-label="Dessert calories">
                    <thead>
                        <tr className="mdc-env-data-table__header-row">
                            <th
                                className="mdc-env-data-indicator mdc-env-data-indicator-over"
                                role="columnheader"
                                scope="col">
                                <div className="mdc-env-data-header-title">Environment Locks</div>
                            </th>
                        </tr>
                        <tr className="mdc-env-data-table__header-row">
                            <th className="mdc-env-data-indicator" role="columnheader" scope="col">
                                <div className="mdc-env-data-indicator-header">
                                    <div className="mdc-env-data-indicator-field">Date</div>
                                    <div className="mdc-env-data-indicator-field">Environment</div>
                                    <div className="mdc-env-data-indicator-field">Lock Id</div>
                                    <div className="mdc-env-data-indicator-field">Message</div>
                                    <div className="mdc-env-data-indicator-field">Author Name</div>
                                    <div className="mdc-env-data-indicator-field">Author Email</div>
                                    <div className="mdc-env-data-indicator-field">Actions</div>
                                </div>
                            </th>
                        </tr>
                    </thead>
                    <tbody className="mdc-env-data-table__content">
                        {envlocks.map((lock) => (
                            <EnvironmentLockDisplay lock={lock}></EnvironmentLockDisplay>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};
