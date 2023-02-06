import React from 'react';
import { act, fireEvent, getByText, render } from '@testing-library/react';
import { Spy } from 'spy4js';
import { BatchAction, Environment_Application_ArgoCD, LockBehavior } from '../../api/api';
import { ActionsCartContext } from '../App';
import { callbacks, CheckoutCart } from './CheckoutDialog';
import { CartAction } from '../ActionDetails';
import { mockGetOverviewResponseForActions } from './apiMock';

const sampleDeployAction: CartAction = {
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
const mock_addMessageToAction = Spy.mockModule('../ActionDetails', 'addMessageToAction');

const mock_setActions = Spy('setActions');
const doActionsSpy = Spy('doActionsSpy');

describe('Checkout Dialog', () => {
    const getNode = (actions: CartAction[], argoCD?: Environment_Application_ArgoCD) => {
        const value = { actions: actions, setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <CheckoutCart overview={mockGetOverviewResponseForActions(actions, argoCD)} />
            </ActionsCartContext.Provider>
        );
    };
    const getWrapper = (actions: CartAction[], argoCD?: Environment_Application_ArgoCD) =>
        render(getNode(actions, argoCD));

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
            cart: [sampleDeployAction],
            transformedCart: [sampleActionTransformed],
            doActionsSucceed: true,
            expect: {
                applyIsDisabled: false,
            },
        },
        {
            type: 'Cart with some action -- doAction fails',
            cart: [sampleDeployAction],
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
            mock_addMessageToAction.addMessageToAction.returns(sampleActionTransformed);
            const { container } = getWrapper(testcase.cart);

            if (testcase.cart.length) mock_addMessageToAction.addMessageToAction.wasCalledWith(testcase.cart[0], '');
            else mock_addMessageToAction.addMessageToAction.wasNotCalled();

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
                mock_addMessageToAction.addMessageToAction.wasCalledWith(testcase.cart[0], 'foo bar');
                mock_useBatch.useBatch.wasCalled();
            } else {
                expect(textField).toBe(null);
            }
        });
    });

    describe.each([
        {
            type: 'Sync windows missing entirely (backward compat)',
            cart: [sampleDeployAction],
            argoCD: undefined,
            wantWarning: false,
        },
        {
            type: 'Without sync windows',
            cart: [sampleDeployAction],
            argoCD: { syncWindows: [] },
            wantWarning: false,
        },
        {
            type: 'With sync windows',
            cart: [sampleDeployAction],
            argoCD: { syncWindows: [{ kind: 'allow', schedule: '* * * * *', duration: '0s' }] },
            wantWarning: true,
        },
    ])(`Checkout and sync window behaviour`, (testcase) => {
        it(`${testcase.type}`, () => {
            // given
            mock_useBatch.useBatch.returns([doActionsSpy, { state: 'waiting' }]);
            mock_addMessageToAction.addMessageToAction.returns(sampleActionTransformed);
            const { container } = getWrapper(testcase.cart, testcase.argoCD);

            // then
            const warning = container.querySelector('.MuiAlert-outlinedWarning');
            if (testcase.wantWarning) {
                expect(warning).toBeInTheDocument();
            } else {
                expect(warning).not.toBeInTheDocument();
            }
        });
    });
});
