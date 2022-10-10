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
import classNames from 'classnames';
import { cloneElement, useEffect, useRef } from 'react';
import { MDCRipple } from '@material/ripple';
import { Link, useLocation } from 'react-router-dom';

export const NavbarIndicator = (props: { pathname: string; to: string }) => {
    const { pathname, to } = props;
    return (
        <div
            className={classNames(
                'mdc-list-item-indicator',
                pathname.startsWith(`/v2/${to}`) ? 'mdc-list-item-indicator--activated' : ''
            )}></div>
    );
};

export const NavListItem = (props: { className?: string; to: string; search?: string; icon?: JSX.Element }) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLAnchorElement>(null);
    const { className, to, search, icon } = props;
    const { pathname } = useLocation();

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <div className="mdc-list-item-container">
            <NavbarIndicator pathname={pathname} to={to} />
            <Link
                aria-label={to}
                className={classNames(
                    'mdc-list-item',
                    pathname.startsWith(`/v2/${to}`) ? 'mdc-list-item--activated' : '',
                    className
                )}
                ref={control}
                to={to + search}
                tabIndex={pathname.startsWith(`/v2/${to}`) ? 0 : -1}>
                <div className="mdc-list-item__ripple" />
                {icon &&
                    cloneElement(icon, {
                        key: 'icon',
                    })}
            </Link>
        </div>
    );
};
