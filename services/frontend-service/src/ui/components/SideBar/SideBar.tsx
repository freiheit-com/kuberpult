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
import { HideBarWhite } from '../../../images';

export const SideBar: React.FC<{ className: string; toggleSidebar: () => void }> = (props) => {
    const { className, toggleSidebar } = props;

    return (
        <aside className={className}>
            <nav className="mdc-drawer-sidebar mdc-drawer__drawer sidebar-content">
                <div className="mdc-drawer-sidebar mdc-drawer-sidebar-header">
                    <Button
                        className={'mdc-drawer-sidebar mdc-drawer-sidebar-header mdc-drawer-sidebar-header__button'}
                        icon={<HideBarWhite />}
                        onClick={toggleSidebar}
                    />
                    <h1 className="mdc-drawer-sidebar mdc-drawer-sidebar-header-title">Planned Actions</h1>
                </div>
                <nav className="mdc-drawer-sidebar mdc-drawer-sidebar-content">
                    <div className="mdc-drawer-sidebar mdc-drawer-sidebar-list">
                        <div>{'Action 1'}</div>

                        <div>{'Action 2'}</div>

                        <div>{'Action 3'}</div>

                        <div>{'Action 4'}</div>
                    </div>
                </nav>
                <div className="mdc-drawer-sidebar mdc-sidebar-sidebar-footer">
                    <Button
                        className="mdc-drawer-sidebar mdc-sidebar-sidebar-footer mdc-drawer-sidebar-apply-button"
                        label={'Apply'}
                    />
                </div>
            </nav>
        </aside>
    );
};
