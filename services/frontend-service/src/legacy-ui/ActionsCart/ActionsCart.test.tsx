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
import { fireEvent, getByText, render } from '@testing-library/react';
import { ActionsCart } from './ActionsCart';
import { Spy } from 'spy4js';
import { ActionsCartContext } from '../App';
import { mockGetOverviewResponseForActions } from './apiMock';
import { CartAction } from '../ActionDetails';

Spy.mockReactComponents('./CheckoutDialog', 'CheckoutCart');
const mock_setActions = Spy('setActions');

describe('Actions Cart', () => {
    const getNode = (actions: CartAction[]) => {
        const value = { actions: actions, setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <ActionsCart overview={mockGetOverviewResponseForActions(actions)} />
            </ActionsCartContext.Provider>
        );
    };
    const getWrapper = (actions: CartAction[]) => render(getNode(actions));

    interface dataT {
        type: string;
        cart: CartAction[];
        expect: {
            cartEmptyMessage?: string;
        };
    }

    const data: dataT[] = [
        {
            type: 'Multiple cart actions',
            cart: [
                {
                    deploy: {
                        application: 'dummy application',
                        version: 22,
                        environment: 'dummy environment',
                    },
                },
                {
                    createEnvironmentLock: {
                        environment: 'dummy environment',
                    },
                },
                {
                    createApplicationLock: {
                        application: 'dummy application',
                        environment: 'dummy environment',
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
            const { container } = getWrapper(testcase.cart);

            // when rendered
            expect(getByText(container, /planned actions/i)).toBeTruthy();

            // then
            const list = document.querySelector('.actions');
            expect(list?.children.length).toBe(testcase.cart.length);

            // when
            if (testcase.cart.length === 0) {
                expect(document.querySelector('.cart-drawer')?.textContent).toContain(testcase.expect.cartEmptyMessage);
            } else {
                // when deleting an item from cart
                const item1 = list?.children[1];
                fireEvent.click(item1?.querySelector('button')!);

                // then
                mock_setActions.wasCalledWith(testcase.cart.filter((_, i) => i !== 1));
            }
        });
    });
});
