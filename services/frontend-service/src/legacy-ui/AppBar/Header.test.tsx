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
import React from 'react';
import { getByTestId, render } from '@testing-library/react';
import { HeaderTitle } from './Header';

describe('Show Kuberpult Version', () => {
    interface dataT {
        name: string;
        tooltipText: string;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'renders the Tooltip component without version',
            tooltipText: '',
            expect: (container) =>
                expect(getByTestId(container, 'kuberpult-version')).toHaveAttribute('aria-label', 'Kuberpult '),
        },
        {
            name: 'renders the Tooltip component with version',
            tooltipText: '1.0.0',
            expect: (container) =>
                expect(getByTestId(container, 'kuberpult-version')).toHaveAttribute('aria-label', 'Kuberpult 1.0.0'),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <HeaderTitle {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { kuberpultVersion: string }) => render(getNode(overrides));

    describe.each(data)(`Kuberpult Version UI`, (testcase) => {
        it(testcase.name, () => {
            const { tooltipText } = testcase;
            // when
            const { container } = getWrapper({ kuberpultVersion: tooltipText });
            // then
            testcase.expect(container);
        });
    });
});
