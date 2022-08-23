/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import React from 'react';
import { render } from '@testing-library/react';
import { NavbarIndicator } from './navListItem';

describe('Display sidebar indicator', () => {
    interface dataT {
        name: string;
        pathname: string;
        to: string;
        expect: (container: HTMLElement, url?: string) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'Indicator is not displayed',
            pathname: '/v2/test/',
            to: 'anotherTest',
            expect: (container) =>
                expect(container.querySelector(`.mdc-list-item-indicator--activated`)).not.toBeTruthy(),
        },
        {
            name: 'Indicator is displayed',
            pathname: '/v2/test/',
            to: 'test',
            expect: (container) => expect(container.querySelector(`.mdc-list-item-indicator--activated`)).toBeTruthy(),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <NavbarIndicator {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { pathname: string; to: string }) => render(getNode(overrides));

    describe.each(data)(`Sidebar Indicator Cases`, (testcase) => {
        it(testcase.name, () => {
            const { pathname, to } = testcase;
            // when
            const { container } = getWrapper({ pathname: pathname, to: to });
            // then
            testcase.expect(container);
        });
    });
});
