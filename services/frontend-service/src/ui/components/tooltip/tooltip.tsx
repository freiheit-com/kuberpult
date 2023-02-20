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
import { useEffect, useRef } from 'react';
import { MDCTooltip } from '@material/tooltip';

export const Tooltip = (props: { children: JSX.Element; content: JSX.Element; id: string }): JSX.Element => {
    const MDComponent = useRef<MDCTooltip>();
    const control = useRef<HTMLDivElement>(null);
    const { children, content, id } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCTooltip(control.current);
        }
        return (): void => MDComponent.current?.destroy();
    }, []);

    return (
        <div className="mdc-tooltip-wrapper--rich">
            <div
                data-tooltip-id={'tt-' + id}
                className="mdc-tooltip__container"
                aria-haspopup="dialog"
                aria-expanded="false">
                {children}
            </div>
            <div
                id={'tt-' + id}
                ref={control}
                className="mdc-tooltip mdc-tooltip--rich"
                aria-hidden="true"
                tabIndex={-1}
                role="dialog">
                <div className="mdc-tooltip__surface mdc-tooltip__surface-animation">
                    <div className="mdc-tooltip__caret-surface-top" />
                    <div className="mdc-tooltip__content">{content}</div>
                </div>
            </div>
        </div>
    );
};
