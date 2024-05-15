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
import React from 'react';
import { render } from '@testing-library/react';
import { Tooltip } from './tooltip';

describe('Tooltip', () => {
    const getNode = (): JSX.Element => (
        <Tooltip
            tooltipContent={<div>I'm inside the tooltip</div>}
            children={<button>click me</button>}
            id={'test-tooltip'}
        />
    );
    const getWrapper = () => render(getNode());

    it(`Renders a tooltip`, () => {
        // when
        const { container } = getWrapper();
        // then
        expect(container.firstChild).toMatchSnapshot();
    });
});
