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
import { Tooltip as TooltipReact } from 'react-tooltip';

export const Tooltip = (props: { children: JSX.Element; tooltipContent: JSX.Element; id: string }): JSX.Element => {
    const { children, tooltipContent, id } = props;
    const delayHide = 50; // for debugging the css, increase this to 1000000

    // The React tooltip really wants us to use a href, but also we don't want to do anything on click:
    const href = '#';
    return (
        <div className={'tooltip-container'}>
            <a href={href} id={'tooltip' + id} data-tooltip-place="bottom" data-tooltip-delay-hide={delayHide}>
                {children}
            </a>
            <TooltipReact className={'tooltip'} anchorSelect={'#tooltip' + id} border={'solid 2px lightgray'}>
                {tooltipContent}
            </TooltipReact>
        </div>
    );
};
