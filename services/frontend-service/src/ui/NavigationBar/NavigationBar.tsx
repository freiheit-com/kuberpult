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
import { ReactComponent as Logo } from '../../images/kuberpult-logo.svg';
import { ReactComponent as Home } from '../../images/Home.svg';
import { ReactComponent as Environments } from '../../images/Environments.svg';
import { ReactComponent as Locks } from '../../images/Locks.svg';
import { NavList, NavListItem } from '../components/navigation';

export const NavigationBar: React.FC = () => (
    <aside className="mdc-drawer" id={'NavigationBar'}>
        <div className="kp-logo">
            <Logo />
        </div>
        <div className="mdc-drawer__content">
            <NavList>
                <NavListItem to={'home'} icon={<Home />} />
                <NavListItem to={'environments'} icon={<Environments />} />
                <NavListItem to={'locks'} icon={<Locks />} />
            </NavList>
        </div>
    </aside>
);
