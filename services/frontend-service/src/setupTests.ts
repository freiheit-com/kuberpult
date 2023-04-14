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
import '@testing-library/jest-dom/extend-expect';
import 'react-use-sub/test-util';

// test utility to await all running promises
global.nextTick = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));

export const documentQuerySelectorSafe = (selectors: string): HTMLElement => {
    const result = document.querySelector(selectors);
    if (!result) {
        throw new Error('did not find in selector in document ' + selectors);
    }
    if (!(result instanceof HTMLElement)) {
        throw new Error('did find element in selector but it is not an html element: ' + selectors);
    }
    return result;
};

export const elementQuerySelectorSafe = (element: HTMLElement, selectors: string): HTMLElement => {
    const result = element.querySelector(selectors);
    if (!result) {
        throw new Error('did not find in selector in document ' + selectors);
    }
    if (!(result instanceof HTMLElement)) {
        throw new Error('did find element in selector but it is not an html element: ' + selectors);
    }
    return result;
};
