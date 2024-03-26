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
import ReactDOMServer from 'react-dom/server';

// As long as there is only a single global provider, there is no need to sync its id manually.
// We currently only provide styling for the default tooltip.
export const tooltipProviderGlobalID = 'kuberpult-tooltip';

export type TooltipProps = {
    children: JSX.Element;
    id: string;
    tooltipContent: JSX.Element;
    tooltipProviderID: string;
};

export const Tooltip = (overrides?: Partial<TooltipProps>): JSX.Element => {
    const props: TooltipProps = {
        tooltipProviderID: tooltipProviderGlobalID,
        children: <></>,
        id: '',
        tooltipContent: <></>,
        ...overrides,
    };

    const { children, tooltipContent, id, tooltipProviderID } = props;
    const delayHide = 50; // for debugging the css, increase this to 1000000

    // The React tooltip really wants us to use a href, but also we don't want to do anything on click:
    return (
        <div
            className={'tooltip-container'}
            id={'tooltip' + id}
            data-tooltip-place="bottom"
            data-tooltip-delay-hide={delayHide}
            data-tooltip-id={tooltipProviderID}
            data-tooltip-html={ReactDOMServer.renderToStaticMarkup(tooltipContent)}>
            {children}
        </div>
    );
};

export type TooltipProviderProps = { id: string; className: string };

// The tooltip provider handles displaying of all tool tips with one sprt of styling.
// It should always be rendered and hence present at root of the DOM (preferably in App.tsx).
export const TooltipProvider = (overwrites?: Partial<TooltipProviderProps>): JSX.Element => {
    const props: TooltipProviderProps = { className: 'tooltip', id: tooltipProviderGlobalID, ...overwrites };
    return <TooltipReact className={props.className} id={props.id} border={'2px solid lightgray'}></TooltipReact>;
};
