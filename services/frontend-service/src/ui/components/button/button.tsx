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
import { useRef, useEffect, useCallback } from 'react';
import classNames from 'classnames';
import { MDCRipple } from '@material/ripple';

export const Button = (props: { className?: string; label?: string; icon?: string; onClick?: any }) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLButtonElement>(null);
    const { className, label, icon, onClick } = props;

    const toClick = useCallback(onClick, [onClick]);

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <button
            data-testid={'display-sideBar'}
            className={classNames('mdc-button', className)}
            onClick={toClick}
            ref={control}
            aria-label={label || ''}>
            <div className="mdc-button__ripple" />
            {!!icon && (
                <i className="medium material-icons mdc-list-item__graphic" aria-hidden="true">
                    {icon}
                </i>
            )}
            {!!label && (
                <span key="label" className="mdc-button__label">
                    {label}
                </span>
            )}
        </button>
    );
};
