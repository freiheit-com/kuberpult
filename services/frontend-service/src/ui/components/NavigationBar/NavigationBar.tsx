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

Copyright 2023 freiheit.com*/
import { Logo, Home, Environments, LocksWhite } from '../../../images';
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
                    <NavListItem to={'home'} queryParams={location?.search || ''} icon={<Home />} />
                    <NavListItem to={'environments'} queryParams={location?.search || ''} icon={<Environments />} />
                    <NavListItem to={'locks'} queryParams={location?.search || ''} icon={<LocksWhite />} />
                </NavList>
            </div>
        </aside>
    );
};
