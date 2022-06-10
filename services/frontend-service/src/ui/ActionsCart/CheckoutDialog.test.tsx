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
import { BatchAction, Environment_Application_SyncWindow, LockBehavior } from '../../api/api';
import { ActionsCartContext } from '../App';
import { callbacks, CheckoutCart } from './CheckoutDialog';
import { Context } from '../Api';
import { makeApiMock } from './apiMock';

const mock_useBatch = Spy.mock(callbacks, 'useBatch');

const mock_setActions = Spy('setActions');
const doActionsSpy = Spy('doActionsSpy');

describe('Checkout Dialog', () => {
    const getNode = (
        actions: BatchAction[],
        syncWindows: Environment_Application_SyncWindow[],
        getOverviewState: 'pending' | 'resolved' | 'rejected'
    ) => {
        const value = { actions: actions, setActions: mock_setActions };
        return (
            <Context.Provider value={makeApiMock(actions, syncWindows, getOverviewState)}>
                <ActionsCartContext.Provider value={value}>
                    <CheckoutCart />
                </ActionsCartContext.Provider>
            </Context.Provider>
        );
    };
    const getWrapper = (
        actions: BatchAction[],
        syncWindows: Environment_Application_SyncWindow[],
        getOverviewState: 'pending' | 'resolved' | 'rejected'
    ) => render(getNode(actions, syncWindows, getOverviewState));

    interface dataT {
        type: string;
        cart: BatchAction[];
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
        },
        {
            type: 'No actions',
            cart: [],
        },
    ];

    describe.each(data)(`Checkout with`, (testcase: dataT) => {
        it(`${testcase.type}`, async () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);

            const { container } = getWrapper(testcase.cart, [], 'resolved');
            await act(global.nextTick);

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

    describe.each([
        {
            type: 'Without sync windows but request pending',
            cart: [
                {
                    action: {
                        $case: 'deploy' as const,
                        deploy: {
                            application: 'test application',
                            environment: 'test environment',
                            version: 0,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.UNRECOGNIZED,
                        },
                    },
                },
            ],
            state: 'pending' as const,
            syncWindows: [],
            wantSpinner: true,
            wantWarning: false,
            wantError: false,
        },
        {
            type: 'Without sync windows and request resolved',
            cart: [
                {
                    action: {
                        $case: 'deploy' as const,
                        deploy: {
                            application: 'test application',
                            environment: 'test environment',
                            version: 0,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.UNRECOGNIZED,
                        },
                    },
                },
            ],
            state: 'resolved' as const,
            syncWindows: [],
            wantSpinner: false,
            wantWarning: false,
            wantError: false,
        },
        {
            type: 'With sync windows and request resolved',
            cart: [
                {
                    action: {
                        $case: 'deploy' as const,
                        deploy: {
                            application: 'test application',
                            environment: 'test environment',
                            version: 0,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.UNRECOGNIZED,
                        },
                    },
                },
            ],
            state: 'resolved' as const,
            syncWindows: [{ kind: 'allow', schedule: '* * * * *', duration: '0s' }],
            wantSpinner: false,
            wantWarning: true,
            wantError: false,
        },
        {
            type: 'Without sync windows and request failed',
            cart: [
                {
                    action: {
                        $case: 'deploy' as const,
                        deploy: {
                            application: 'test application',
                            environment: 'test environment',
                            version: 0,
                            ignoreAllLocks: false,
                            lockBehavior: LockBehavior.UNRECOGNIZED,
                        },
                    },
                },
            ],
            state: 'rejected' as const,
            syncWindows: [],
            wantSpinner: false,
            wantWarning: false,
            wantError: true,
        },
    ])(`Checkout and sync window behaviour`, (testcase) => {
        it(`${testcase.type}`, async () => {
            // given
            const { container } = getWrapper(testcase.cart, testcase.syncWindows, testcase.state);
            await act(global.nextTick);

            // then
            const spinner = container.querySelector('.MuiCircularProgress-root');
            if (testcase.wantSpinner) {
                expect(spinner).toBeInTheDocument();
            } else {
                expect(spinner).not.toBeInTheDocument();
            }

            const warning = container.querySelector('.MuiAlert-outlinedWarning');
            if (testcase.wantWarning) {
                expect(warning).toBeInTheDocument();
            } else {
                expect(warning).not.toBeInTheDocument();
            }

            const error = container.querySelector('.MuiAlert-outlinedError');
            if (testcase.wantError) {
                expect(error).toBeInTheDocument();
            } else {
                expect(error).not.toBeInTheDocument();
            }
        });
    });
});
