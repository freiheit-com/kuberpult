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
    }

    const data: dataT[] = [
        {
            type: 'Some action',
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
        },
        {
            type: 'No actions',
            cart: [],
        },
    ];

    describe.each(data)(`Checkout with`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);
            const { container } = getWrapper(testcase.cart);

            const applyButton = getByText(container, /apply/i).closest('button');
            if (testcase.cart.length === 0) {
                expect(applyButton).toBeDisabled();
            } else {
                // when open dialog
                expect(applyButton).not.toBeDisabled();
                fireEvent.click(applyButton!);

                // then
                mock_useBatch.useBatch.wasCalledWith(testcase.cart, Spy.IGNORE, Spy.IGNORE);

                // when click yes
                const d = document.querySelector('.MuiDialog-root');
                const y = getByText(d! as HTMLElement, /yes/i).closest('button');
                fireEvent.click(y!);

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
