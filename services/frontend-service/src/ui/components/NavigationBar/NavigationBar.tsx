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
import { Logo, Home, Locks, Environments } from '../../../images';
import { NavList, NavListItem } from '../navigation';
import { useLocation } from 'react-router-dom';

export const NavigationBar: React.FC = () => {
    const location = useLocation();
    return (
        <aside className="mdc-drawer">
            <div className="kp-logo">
                <Logo />
            </div>
            <div className="mdc-drawer__content">
                <NavList>
                    <NavListItem to={'home'} search={location.search ? location.search : ''} icon={<Home />} />
                    <NavListItem
                        to={'environments'}
                        search={location.search ? location.search : ''}
                        icon={<Environments />}
                    />
                    <NavListItem to={'locks'} search={location.search ? location.search : ''} icon={<Locks />} />
                </NavList>
            </div>
        </aside>
    );
};
