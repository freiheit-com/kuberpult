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

export const Card = (props: { className?: string; children?: React.ReactElement }) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLDivElement>(null);
    const { className, children } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <div className={classNames('mdc-card', className)}>
            <div className="mdc-card__primary-action" ref={control} tabIndex={0}>
                <div className="mdc-card__ripple"></div>
                <div className="mdc-card-content">
                    Hello
                    {children && cloneElement(children)}
                </div>
            </div>
        </div>
    );
};
