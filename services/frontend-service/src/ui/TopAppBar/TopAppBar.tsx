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

export const TopAppBar: React.FC = () => (
    <div className="mdc-top-app-bar">
        <div className="mdc-top-app-bar__row">
            <section className="mdc-top-app-bar__section mdc-top-app-bar__section--align-start">
                <span className="mdc-top-app-bar__title">Kuberpult</span>
            </section>
            <section className="mdc-top-app-bar__section text-field-container">
                <label className="mdc-text-field mdc-text-field--outlined mdc-text-field--no-label">
                    <span className="mdc-notched-outline">
                        <span className="mdc-notched-outline__leading"></span>
                        <span className="mdc-notched-outline__trailing"></span>
                    </span>
                    <input className="mdc-text-field__input" type="text" aria-label="Search" />
                </label>
            </section>
            <section className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                <button className="mdc-button mdc-top-app-bar__action-item">
                    <span className="mdc-button__ripple"></span>
                    Planned Actions
                </button>
            </section>
        </div>
    </div>
);
