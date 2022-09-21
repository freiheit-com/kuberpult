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
import { useApplicationLocks } from '../../utils/store';
import { ApplicationLockDisplay } from '../ApplicationLockDisplay/ApplicationLockDisplay';

export const ApplicationLocksCard: React.FC<{}> = (props) => {
    const appLocks = useApplicationLocks();
    // eslint-disable-next-line no-console
    console.log(appLocks);
    return (
        <div className="mdc-app-data-table">
            <div className="mdc-app-data-table__table-container">
                <table className="mdc-app-data-table__table" aria-label="Dessert calories">
                    <thead>
                        <tr className="mdc-app-data-table__header-row">
                            <th className="mdc-app-data-indicator" role="columnheader" scope="col">
                                <div className="mdc-app-data-header-title">Application Locks</div>
                            </th>
                        </tr>
                        <tr className="mdc-app-data-table__header-row">
                            <th
                                className="mdc-app-data-indicator mdc-app-data-indicator--subheader"
                                role="columnheader"
                                scope="col">
                                <div className="mdc-app-data-indicator-header">
                                    <div className="mdc-app-data-indicator-field">Date</div>
                                    <div className="mdc-app-data-indicator-field">Application</div>
                                    <div className="mdc-app-data-indicator-field">Environment</div>
                                    <div className="mdc-app-data-indicator-field">Lock Id</div>
                                    <div className="mdc-app-data-indicator-field">Message</div>
                                    <div className="mdc-app-data-indicator-field">Author Name</div>
                                    <div className="mdc-app-data-indicator-field">Author Email</div>
                                    <div className="mdc-app-data-indicator-field">Actions</div>
                                </div>
                            </th>
                        </tr>
                    </thead>
                    <tbody className="mdc-app-data-table__content">
                        {appLocks.map((lock) => (
                            <ApplicationLockDisplay lock={lock}></ApplicationLockDisplay>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};
