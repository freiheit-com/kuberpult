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
import { CartAction } from '../ActionDetails';

const sampleAction: CartAction = {
    deploy: {
        application: 'dummy application',
        version: 22,
        environment: 'dummy environment',
    },
};

const sampleActionTransformed: BatchAction = {
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
};

const sampleLockAction: CartAction = {
    createEnvironmentLock: {
        environment: 'dummy environment',
    },
};

const mock_useBatch = Spy.mock(callbacks, 'useBatch');
const mock_transformToBatch = Spy.mockModule('../ActionDetails', 'transformToBatch');

const mock_setActions = Spy('setActions');
const doActionsSpy = Spy('doActionsSpy');

describe('Checkout Dialog', () => {
    const getNode = (actions: CartAction[]) => {
        const value = { actions: actions, setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <CheckoutCart />
            </ActionsCartContext.Provider>
        );
    };
    const getWrapper = (actions: CartAction[]) => render(getNode(actions));

    interface dataT {
        type: string;
        cart: CartAction[];
        transformedCart: BatchAction[];
        doActionsSucceed?: boolean;
        lockAction?: boolean;
        expect: {
            applyIsDisabled: boolean;
        };
    }

    const data: dataT[] = [
        {
            type: 'Cart with some action -- doAction succeeds',
            cart: [sampleAction],
            transformedCart: [sampleActionTransformed],
            doActionsSucceed: true,
            expect: {
                applyIsDisabled: false,
            },
        },
        {
            type: 'Cart with some action -- doAction fails',
            cart: [sampleAction],
            transformedCart: [sampleActionTransformed],
            doActionsSucceed: false,
            expect: {
                applyIsDisabled: false,
            },
        },
        {
            type: 'Cart with Lock action',
            cart: [sampleLockAction],
            lockAction: true,
            transformedCart: [sampleActionTransformed], // transformToBatch is mocked
            expect: {
                applyIsDisabled: true,
            },
        },
        {
            type: 'cart with no actions',
            cart: [],
            transformedCart: [],
            expect: {
                applyIsDisabled: true,
            },
        },
    ];

    describe.each(data)(`Checkout`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);
            mock_transformToBatch.transformToBatch.returns(sampleActionTransformed);
            const { container } = getWrapper(testcase.cart);

            if (testcase.cart.length) mock_transformToBatch.transformToBatch.wasCalledWith(testcase.cart[0], '');
            else mock_transformToBatch.transformToBatch.wasNotCalled();

            if (testcase.cart.length)
                mock_useBatch.useBatch.wasCalledWith(testcase.transformedCart, Spy.IGNORE, Spy.IGNORE);
            else mock_useBatch.useBatch.wasCalledWith([], Spy.IGNORE, Spy.IGNORE);

            const applyButton = getByText(container, /apply/i).closest('button');
            const textField = container.querySelector('.actions-cart__lock-message input');

            if (testcase.expect.applyIsDisabled) {
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

                if (testcase.doActionsSucceed) {
                    // when doActions succeed, do the actions that useBatch is expected to do
                    act(() => {
                        mock_useBatch.useBatch.getCallArgument(0, 1)();
                    });

                    // then
                    mock_setActions.wasCalledWith([]);
                } else {
                    // when doActions fails, do the actions that useBatch is expected to do
                    act(() => {
                        mock_useBatch.useBatch.getCallArgument(0, 2)();
                    });

                    // then
                    mock_setActions.wasNotCalled();
                }

                // when - there's a create-lock action
                if (testcase.lockAction) {
                    // then
                    expect(applyButton).toBeDisabled();
                    expect(textField).toBeTruthy();

                    // when - adding a lock message
                    fireEvent.change(textField!, { target: { value: 'foo bar' } });

                    // then - useBatch is called with updated actions
                    expect(applyButton).not.toBeDisabled();
                    mock_transformToBatch.transformToBatch.wasCalledWith(testcase.cart[0], 'foo bar');
                    mock_useBatch.useBatch.wasCalled();
                } else {
                    expect(textField).toBe(null);
                }
            }
        });
    });
});
