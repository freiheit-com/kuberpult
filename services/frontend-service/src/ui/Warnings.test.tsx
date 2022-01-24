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
import { getByLabelText, getByText, render } from '@testing-library/react';
import { UndeployBtn } from './Warnings';

describe('Undeploy Button', () => {
    interface dataT {
        name: string;
        state?: string;
        selector: (container: HTMLElement) => HTMLElement | null;
    }

    const data: dataT[] = [
        {
            name: 'renders the UndeployBtn component',
            state: 'waiting',
            selector: (container) => getByLabelText(container, /This app is ready to un-deploy./i),
        },
        {
            name: 'renders the UndeployBtn component with undefined state',
            state: 'undefined state',
            selector: (container) => getByText(container, /Unknown/i),
        },
        {
            name: 'renders the UndeployBtn component with resolved state',
            state: 'resolved',
            selector: (container) => container.querySelector('.Mui-disabled'),
        },
        {
            name: 'renders the UndeployBtn component with pending state',
            state: 'pending',
            selector: (container) => container.querySelector('.MuiCircularProgress-root'),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
            state: 'no state', //
            applicationName: 'app1', //
        };
        return <UndeployBtn {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { state?: string }) => render(getNode(overrides));

    describe.each(data)(`Undeploy Button with state`, (testcase) => {
        it(testcase.name, () => {
            // when
            const { container } = getWrapper({ state: testcase.state });
            // then
            expect(testcase.selector(container)).toBeTruthy();
        });
    });
});
