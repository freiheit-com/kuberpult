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
import { render, getAllByTestId } from '@testing-library/react';
import { TopAppBar } from '../TopAppBar/TopAppBar';

describe('Show and Hide Sidebar', () => {
    interface dataT {
        name: string;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'Sidebar is hidden',
            expect: (container) =>
                expect(container.getElementsByClassName('mdc-drawer-sidebar--hidden')[0]).toBeTruthy(),
        },
        {
            name: 'Sidebar is displayed',
            expect: (container) => {
                const result = getAllByTestId(container, 'display-sideBar')[0];
                result.click();
                expect(container.getElementsByClassName('mdc-drawer-sidebar--displayed')[0]).toBeTruthy();
            },
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <TopAppBar {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: {}) => render(getNode(overrides));

    describe.each(data)(`SideBar functionality`, (testcase) => {
        it(testcase.name, () => {
            // when
            const { container } = getWrapper({});
            // then
            testcase.expect(container);
        });
    });
});
