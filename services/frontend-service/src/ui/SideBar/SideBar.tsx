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
import { Button } from '../components/button';

export const SideBar: React.FC<{ className: string; toggleSidebar: () => void }> = (props) => {
    const { className, toggleSidebar } = props;

    return (
        <aside className={className}>
            <nav className="mdc-drawer__drawer sidebar-content">
                <div className="sidebar-header">
                    <Button className="mdc-top-button" icon={'navigate_next'} onClick={toggleSidebar} />
                    <h1 className="sidebar-header-title">Planned Actions</h1>
                </div>
                <nav className="mdc-drawer-content">
                    <div id="icon-with-text-demo" className="mdc-list">
                        <div>{'Action 1'}</div>

                        <div>{'Action 2'}</div>

                        <div>{'Action 3'}</div>

                        <div>{'Action 4'}</div>
                    </div>
                </nav>
            </nav>
            <div className="sidebar-footer">
                <button className="sidebar-footer-button">Apply</button>
            </div>
        </aside>
    );
};
