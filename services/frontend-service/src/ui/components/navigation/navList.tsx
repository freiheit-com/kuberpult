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
import { ReactNode, useEffect, useRef } from 'react';
import { MDCList } from '@material/list';

export const NavList: React.FC<{ children?: ReactNode; className?: string }> = (props) => {
    const MDComponent = useRef<MDCList>();
    const control = useRef<HTMLElement>(null);
    const { className, children } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCList(control.current);
            MDComponent.current.wrapFocus = true;
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <nav className={classNames('mdc-list', className)} ref={control}>
            {children}
        </nav>
    );
};
