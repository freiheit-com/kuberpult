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
import * as React from 'react';
import { Button } from '../button';
import { useState } from 'react';

export type DotsMenuButton = {
    label: string;
    onClick: () => void;
    icon?: JSX.Element;
};

export type DotsMenuProps = {
    buttons: DotsMenuButton[];
};

export const DotsMenu: React.FC<DotsMenuProps> = (props) => {
    const [open, setOpen] = useState(false);
    const openMenu = React.useCallback(() => {
        setOpen(true);
    }, []);
    const closeMenu = React.useCallback(() => {
        setOpen(false);
    }, []);
    const memoizedOnClick = React.useCallback(
        (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {
            const index = e.currentTarget.id;
            props.buttons[Number(index)].onClick();
            setOpen(false);
        },
        [props.buttons]
    );

    if (!open) {
        return (
            <div className={'dots-menu dots-menu-hidden'}>
                <Button className="mdc-button--unelevated" label={'⋮'} onClick={openMenu} />
            </div>
        );
    }

    return (
        <div className={'dots-menu dots-menu-open'} onMouseLeave={closeMenu}>
            <ul className={'list'}>
                {props.buttons.map((button, index) => (
                    <li className={'item'} key={'button-menu-' + String(index)}>
                        <Button
                            id={String(index)}
                            icon={button.icon}
                            className="mdc-button--unelevated"
                            label={button.label}
                            onClick={memoizedOnClick}
                        />
                    </li>
                ))}
            </ul>
        </div>
    );
};
