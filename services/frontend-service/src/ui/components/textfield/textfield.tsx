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
import { ChangeEventHandler, useCallback, useState } from 'react';
import classNames from 'classnames';

export type TextfieldProps = {
    className?: string;
    placeholder?: string;
    value?: string | number;
    leadingIcon?: string;
    onChange?: ChangeEventHandler;
};

export const Textfield = (props: TextfieldProps): JSX.Element => {
    const { className, placeholder, leadingIcon, value, onChange } = props;

    const [hasFocus, setFocus] = useState(false);

    const onFocus = useCallback((): void => setFocus(true), [setFocus]);
    const onBlur = useCallback((): void => setFocus(false), [setFocus]);

    const allClassName = classNames(
        'mdc-text-field',
        'mdc-text-field--outlined',
        'mdc-text-field--no-label',
        {
            'mdc-text-field--with-leading-icon': leadingIcon,
            'mdc-text-field--focused': hasFocus,
        },
        className
    );

    return (
        <div className={allClassName}>
            <span className="mdc-notched-outline">
                <span className="mdc-notched-outline__leading" />
                <span className="mdc-notched-outline__trailing" />
            </span>
            {leadingIcon && (
                <i className="material-icons mdc-text-field__icon mdc-text-field__icon--leading" tabIndex={0}>
                    {leadingIcon}
                </i>
            )}
            <input
                type="search"
                className="mdc-text-field__input"
                defaultValue={value}
                placeholder={placeholder}
                aria-label={placeholder}
                onChange={onChange}
                onFocus={onFocus}
                onBlur={onBlur}
            />
        </div>
    );
};
