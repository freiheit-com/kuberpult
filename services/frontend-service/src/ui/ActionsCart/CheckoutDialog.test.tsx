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
import { Spy } from 'spy4js';
import { BatchAction, LockBehavior } from '../../api/api';
import { ActionsCartContext } from '../App';
import { callbacks, CheckoutCart } from './CheckoutDialog';

const mock_useBatch = Spy.mock(callbacks, 'useBatch');

const mock_setActions = Spy('setActions');
const doActionsSpy = Spy('doActionsSpy');

describe('Checkout Dialog', () => {
    const getNode = (actions?: BatchAction[]) => {
        const value = { actions: actions ?? [], setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <CheckoutCart />
            </ActionsCartContext.Provider>
        );
    };
    const getWrapper = (actions?: BatchAction[]) => render(getNode(actions));

    interface dataT {
        type: string;
        cart: BatchAction[];
        expect: {
            disabled: boolean;
            updatedMessage?: any;
        };
    }

    const data: dataT[] = [
        {
            type: 'Cart with some action',
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
            ],
            expect: {
                disabled: false,
            },
        },
        {
            type: 'Cart with create lock action',
            cart: [
                {
                    action: {
                        $case: 'createEnvironmentLock',
                        createEnvironmentLock: {
                            environment: 'dummy environment',
                            lockId: '1234',
                            message: '',
                        },
                    },
                },
            ],
            expect: {
                disabled: true, // lock message input is still empty
                updatedMessage: [
                    {
                        action: {
                            createEnvironmentLock: {
                                message: 'foo bar',
                            },
                        },
                    },
                ],
            },
        },
        {
            type: 'cart with no actions',
            cart: [],
            expect: {
                disabled: true,
            },
        },
    ];

    describe.each(data)(`Checkout`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);
            const { container } = getWrapper(testcase.cart);

            mock_useBatch.useBatch.wasCalledWith(testcase.cart, Spy.IGNORE, Spy.IGNORE);

            const applyButton = getByText(container, /apply/i).closest('button');
            const textField = container.querySelector('.actions-cart__lock-message input');

            if (testcase.expect.disabled) {
                expect(applyButton).toBeDisabled();
            } else {
                // when open dialog
                expect(applyButton).not.toBeDisabled();
                fireEvent.click(applyButton!);

                // then
                const d = document.querySelector('.MuiDialog-root');
                expect(d).toBeTruthy();

                // when click yes
                const y = getByText(d! as HTMLElement, /yes/i).closest('button');
                fireEvent.click(y!);

                // then
                doActionsSpy.wasCalled();

                // when do the actions that useBatch is expected to do
                act(() => {
                    mock_useBatch.useBatch.getCallArgument(0, 1)();
                });
                // then
                mock_setActions.wasCalledWith([]);
            }

            // when - there's a create-lock action
            if (testcase.expect.updatedMessage) {
                // then
                expect(applyButton).toBeDisabled();
                expect(textField).toBeTruthy();

                // when - adding a lock message
                fireEvent.change(textField!, { target: { value: 'foo bar' } });

                // then - useBatch is called with updated actions
                expect(applyButton).not.toBeDisabled();
                const calledActions = mock_useBatch.useBatch.getCallArgument(1, 0);
                expect(calledActions).toMatchObject(testcase.expect.updatedMessage);
            } else {
                expect(textField).toBe(null);
            }
        });
    });
});
