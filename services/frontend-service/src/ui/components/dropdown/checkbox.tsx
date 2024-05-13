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
import { Button } from '../button';
import { useCallback } from 'react';

export type CheckboxProps = {
    onClick?: (id: string) => void;
    classes?: string;
    id: string;
    enabled: boolean;
    label: string;
};

export const Checkbox: React.FC<CheckboxProps> = (props) => {
    const onClick = useCallback(() => {
        if (props.onClick) {
            props.onClick(props.id);
        }
    }, [props]);
    return (
        <label>
            <div className={'checkbox-wrapper'}>
                <Button
                    onClick={onClick}
                    className={'test-button-checkbox id-' + props.id + ' ' + (props.enabled ? 'enabled' : 'disabled')}
                    label={props.enabled ? '☑' : '☐'}
                    highlightEffect={false}
                />
                {props.label}
            </div>
        </label>
    );
};
