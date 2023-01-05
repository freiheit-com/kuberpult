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
import { useRef, useEffect, cloneElement } from 'react';
import classNames from 'classnames';
import { MDCRipple } from '@material/ripple';

export const Button = (props: {
    disabled?: boolean;
    className?: string;
    label?: string;
    icon?: JSX.Element;
    onClick?: () => void;
}) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLButtonElement>(null);
    const { disabled, className, label, icon, onClick } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <button
            disabled={disabled}
            className={classNames('mdc-button', className)}
            onClick={onClick}
            ref={control}
            aria-label={label || ''}>
            <div className="mdc-button__ripple" />
            {icon &&
                cloneElement(icon, {
                    key: 'icon',
                })}
            {!!label && (
                <span key="label" className="mdc-button__label">
                    {label}
                </span>
            )}
        </button>
    );
};
