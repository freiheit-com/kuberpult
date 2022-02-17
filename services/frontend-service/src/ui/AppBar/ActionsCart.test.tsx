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
import { act, fireEvent, getByText, render } from '@testing-library/react';
import { callbacks } from '../Batch';
import { Spy } from 'spy4js';
import { BatchAction, LockBehavior } from '../../api/api';
import { ActionsCartContext } from '../App';
import { ActionsCart } from './ActionsCart';

const mock_useBatch = Spy.mock(callbacks, 'useBatch');

const mock_setActions = Spy('setActions');
const doActionsSpy = Spy('doActionsSpy');

describe('Actions Cart', () => {
    const getNode = (actions?: BatchAction[]) => {
        const value = { actions: actions ?? [], setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <ActionsCart />
            </ActionsCartContext.Provider>
        );
    };
    const getWrapper = (actions?: BatchAction[]) => render(getNode(actions));

    interface dataT {
        type: string;
        cart: BatchAction[];
        expect: {
            cartEmptyMessage?: string;
        };
    }

    const data: dataT[] = [
        {
            type: 'Multiple cart actions',
            cart: [
                {
                    action: {
                        $case: 'deploy',
                        deploy: {
                            application: 'dummy application',
                            version: 22,
                            environment: 'dummy environment',
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.Ignore,
                        },
                    },
                },
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: {
                            environment: 'dummy environment',
                            lockId: '1234',
                            message: 'hello',
                        },
                    },
                },
                {
                    action: {
                        $case: 'createEnvironmentApplicationLock',
                        createEnvironmentApplicationLock: {
                            application: 'dummy application',
                            environment: 'dummy environment',
                            lockId: '1111',
                            message: 'hi',
                        },
                    },
                },
            ],
            expect: {},
        },
        {
            type: 'No actions',
            cart: [],
            expect: {
                cartEmptyMessage: 'Cart Is Currently Empty,\nPlease Add Actions!',
            },
        },
    ];

    describe.each(data)(`Cart with`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);
            const { container } = getWrapper(testcase.cart);

            // when rendered
            expect(getByText(container, /planned actions/i)).toBeTruthy();

            // then
            const list = document.querySelector('.actions');
            expect(list?.children.length).toBe(testcase.cart.length);
            mock_useBatch.useBatch.wasCalledWith(testcase.cart, Spy.IGNORE);

            // when
            const a = getByText(document.querySelector('.cart-drawer')! as HTMLElement, /apply/i).closest('button');
            if (testcase.cart.length === 0) {
                expect(document.querySelector('.cart-drawer')?.textContent).toContain(testcase.expect.cartEmptyMessage);
                expect(a).toBeDisabled();
            } else {
                // when deleting an item from cart
                const item1 = list?.children[1];
                fireEvent.click(item1?.querySelector('button')!);

                // then
                mock_setActions.wasCalledWith(testcase.cart.filter((_, i) => i !== 1));

                // when clicking apply
                expect(a).not.toBeDisabled();
                fireEvent.click(a!);

                // then
                doActionsSpy.wasCalled();

                // when do the actions that useBatch is expected to do
                act(() => {
                    mock_useBatch.useBatch.getCallArguments()[1]();
                });
                // then
                mock_setActions.wasCalledWith([]);
            }
        });
    });
});
