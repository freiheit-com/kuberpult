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
import * as React from 'react';

import { PacmanLoader } from 'react-spinners';

export const Spinner: React.FC<{ message: string }> = (props): JSX.Element => {
    const { message } = props;
    return (
        <div className={'spinner'}>
            <div className={'spinner-animation'}>
                <PacmanLoader color={'var(--mdc-theme-primary)'} loading={true} size={100} speedMultiplier={1} />
            </div>
            <div className={'spinner-message'}>{message}...</div>
        </div>
    );
};

export const SmallSpinner: React.FC<{ appName: string; size: number }> = (props): JSX.Element => (
    <div className={'spinner-small'}>
        <div className={'spinner-animation'}>
            <PacmanLoader
                color={'var(--mdc-theme-background)'}
                loading={true}
                size={props.size}
                speedMultiplier={1}
                key={props.appName}
            />
        </div>
    </div>
);
