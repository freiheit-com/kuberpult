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
import { act, render, renderHook } from '@testing-library/react';
import { TopAppBar } from '../TopAppBar/TopAppBar';
import { MemoryRouter } from 'react-router-dom';
import { BatchAction } from '../../../api/api';
import { addAction, updateActions, useActions } from '../../utils/store';

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
                const result = container.querySelector('.mdc-show-button')! as HTMLElement;
                act(() => {
                    result.click();
                });
                expect(container.getElementsByClassName('mdc-drawer-sidebar--displayed')[0]).toBeTruthy();
            },
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return (
            <MemoryRouter>
                <TopAppBar {...defaultProps} {...overrides} />{' '}
            </MemoryRouter>
        );
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

describe('Action Store functionality', () => {
    interface dataT {
        name: string;
        actions: BatchAction[];
        expectedActions: BatchAction[];
    }

    const dataGetSet: dataT[] = [
        {
            name: '1 action',
            actions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
        },
        {
            name: 'Empty action store',
            actions: [],
            expectedActions: [],
        },
        {
            name: '2 different type of actions',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
        },
        {
            name: '2 actions of the same type',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
        },
    ];

    describe.each(dataGetSet)('Test getting actions from the store and setting the store from an array', (testcase) => {
        it(testcase.name, () => {
            // given
            renderHook(() => updateActions(testcase.actions));
            // when
            const actions = renderHook(() => useActions()).result.current;
            expect(actions).toStrictEqual(testcase.expectedActions);
        });
    });

    const dataAdding: dataT[] = [
        {
            name: '1 action',
            actions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
            expectedActions: [{ action: { $case: 'undeploy', undeploy: { application: 'nmww' } } }],
        },
        {
            name: 'Empty action store',
            actions: [],
            expectedActions: [],
        },
        {
            name: '2 different type of actions',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'prepareUndeploy', prepareUndeploy: { application: 'nmww' } } },
            ],
        },
        {
            name: '2 actions of the same type',
            actions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
            expectedActions: [
                { action: { $case: 'undeploy', undeploy: { application: 'nmww' } } },
                { action: { $case: 'undeploy', undeploy: { application: 'auth-service' } } },
            ],
        },
    ];

    describe.each(dataAdding)('Test adding actions to the store', (testcase) => {
        it(testcase.name, () => {
            // given
            renderHook(() => updateActions([]));
            testcase.actions.forEach((action) => {
                renderHook(() => addAction(action));
            });
            // when
            const actions = renderHook(() => useActions()).result.current;
            expect(actions).toStrictEqual(testcase.expectedActions);
        });
    });
});
