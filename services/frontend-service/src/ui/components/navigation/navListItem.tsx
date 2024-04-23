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
import classNames from 'classnames';
import { cloneElement, useRef } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useNavigateWithSearchParams } from '../../utils/store';

export const NavbarIndicator = (props: { pathname: string; to: string }): JSX.Element => {
    const { pathname, to } = props;
    return (
        <div
            className={classNames('mdc-list-item-indicator', {
                'mdc-list-item-indicator--activated': pathname.startsWith(`/${to}`),
            })}></div>
    );
};

export const NavListItem = (props: { className?: string; to: string; icon?: JSX.Element }): JSX.Element => {
    const control = useRef<HTMLAnchorElement>(null);
    const { className, to, icon } = props;
    const { pathname } = useLocation();
    const { navURL } = useNavigateWithSearchParams(to);

    const allClassNames = classNames(
        'mdc-list-item',
        { 'mdc-list-item--activated': pathname.startsWith(`/${to}`) },
        className
    );

    return (
        <div className="mdc-list-item-container">
            <NavbarIndicator pathname={pathname} to={to} />
            <Link
                aria-label={to}
                className={allClassNames}
                ref={control}
                to={navURL}
                tabIndex={pathname.startsWith(`/${to}`) ? 0 : -1}>
                <div className="mdc-list-item__ripple" />
                {icon &&
                    cloneElement(icon, {
                        key: 'icon',
                    })}
            </Link>
        </div>
    );
};
