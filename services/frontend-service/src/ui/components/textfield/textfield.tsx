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
import { useEffect, useRef } from 'react';
import classNames from 'classnames';
import { MDCTextField } from '@material/textfield';

export type TextfieldProps = {
    className?: string;
    floatingLabel?: string;
    value?: string | number;
    leadingIcon?: string;
};

export const Textfield = (props: TextfieldProps) => {
    const { className, floatingLabel, leadingIcon, value } = props;
    const control = useRef<HTMLDivElement>(null);
    const input = useRef<HTMLInputElement>(null);
    const MDComponent = useRef<MDCTextField>();

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCTextField(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    useEffect(() => {
        if (floatingLabel) {
            MDComponent.current?.layout();
        }
    });

    const allClassName = classNames(
        'mdc-text-field',
        'mdc-text-field--outlined',
        {
            'mdc-text-field--no-label': !floatingLabel,
            'mdc-text-field--with-leading-icon': leadingIcon,
        },
        className
    );

    return (
        <div className={allClassName} ref={control}>
            <span className="mdc-notched-outline">
                <span className="mdc-notched-outline__leading" />
                {!!floatingLabel && (
                    <span className="mdc-notched-outline__notch">
                        <span
                            className={classNames('mdc-floating-label', {
                                'mdc-floating-label--float-above': !!value || input.current === document.activeElement,
                            })}>
                            {floatingLabel}
                        </span>
                    </span>
                )}
                <span className="mdc-notched-outline__trailing" />
            </span>
            {leadingIcon && (
                <i
                    className="material-icons mdc-text-field__icon mdc-text-field__icon--leading"
                    tabIndex={0}
                    role="button">
                    {leadingIcon}
                </i>
            )}
            <input type="text" className="mdc-text-field__input" value={value} ref={input} aria-label={floatingLabel} />
        </div>
    );
};
