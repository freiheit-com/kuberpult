/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import { Logo, Home, Environments, LocksWhite } from '../../../images';
import { NavList, NavListItem } from '../navigation';

export const NavigationBar: React.FC = () => (
    <aside className="mdc-drawer">
        <div className="kp-logo">
            <Logo />
        </div>
        <div className="mdc-drawer__content">
            <NavList>
                <NavListItem to={'ui/home'} icon={<Home />} />
                <NavListItem to={'ui/environments'} icon={<Environments />} />
                <NavListItem to={'ui/locks'} icon={<LocksWhite />} />
            </NavList>
        </div>
    </aside>
);
