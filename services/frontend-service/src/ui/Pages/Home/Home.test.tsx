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
import { render } from '@testing-library/react';
import { Home } from './Home';
import { filter, UpdateOverview } from '../../utils/store';
import { Spy } from 'spy4js';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';

const mock_ServiceLane = Spy.mockReactComponents('../../components/ServiceLane/ServiceLane', 'ServiceLane');

describe('App', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <Home />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());
    it('Renders full app', () => {
        // when
        UpdateOverview.set({
            environments: {},
            applications: {
                app1: {} as any,
                app3: {} as any,
                app2: {} as any,
            },
        } as any);
        getWrapper();

        // then apps are sorted and Service Lane is called
        expect(mock_ServiceLane.ServiceLane.getCallArgument(0, 0)).toStrictEqual({ application: 'app1' });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(1, 0)).toStrictEqual({ application: 'app2' });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(2, 0)).toStrictEqual({ application: 'app3' });
    });
});

describe('Application Filter', () => {
    interface dataT {
        name: string;
        query: string;
        applications: string[];
        expect: (nrLocks: number) => void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <MemoryRouter initialEntries={['/one', '/two', { search: 'application' }]}>
                <Home {...defaultProps} {...overrides} />
            </MemoryRouter>
        );
    };
    const getWrapper = (overrides: {}) => render(getNode(overrides));

    const data: dataT[] = [
        {
            name: 'filter applications - 1 result',
            applications: ['dummy', 'test', 'test2', 'foo'],
            query: 'dummy',
            // eslint-disable-next-line no-console
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(1),
        },
        {
            name: 'filter applications - 0 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'foo',
            // eslint-disable-next-line no-console
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(0),
        },
        {
            name: 'filter applications - 2 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'test',
            // eslint-disable-next-line no-console
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(2),
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const nrLocks = testcase.applications.filter((val) => filter(testcase.query, val)).length;
            getWrapper({});
            testcase.expect(nrLocks);
        });
    });
});
