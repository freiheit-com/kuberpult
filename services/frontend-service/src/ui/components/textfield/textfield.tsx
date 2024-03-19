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
import { useCallback } from 'react';
import classNames from 'classnames';
import { useSearchParams } from 'react-router-dom';

export type TextfieldProps = {
    className?: string;
    placeholder?: string;
    value?: string | number;
    leadingIcon?: string;
};

export const Textfield = (props: TextfieldProps): JSX.Element => {
    const { className, placeholder, leadingIcon, value } = props;

    const [searchParams, setSearchParams] = useSearchParams(
        value === undefined || value === '' ? undefined : { application: `${value}` }
    );

    const allClassName = classNames(
        'mdc-text-field',
        'mdc-text-field--outlined',
        'mdc-text-field--no-label',
        {
            'mdc-text-field--with-leading-icon': leadingIcon,
        },
        className
    );

    const setQueryParam = useCallback(
        (event: any) => {
            if (event.target.value !== '') searchParams.set('application', event.target.value);
            else searchParams.delete('application');
            setSearchParams(searchParams);
        },
        [searchParams, setSearchParams]
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
                value={searchParams.get('application') ?? ''}
                placeholder={placeholder}
                aria-label={placeholder}
                onChange={setQueryParam}
            />
        </div>
    );
};
