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
import { fireEvent, getByText, render } from '@testing-library/react';
import { ConfirmationDialogProvider, ConfirmationDialogProviderProps, exportedForTesting } from './ConfirmationDialog';
import { Button } from '@material-ui/core';
import { Spy } from 'spy4js';
import { BatchAction, Lock, LockBehavior } from '../api/api';
import { ActionsCartContext } from './App';

const mock_setActions = Spy('setActions');
const finallySpy = Spy('.finally');

const sampleDeployAction: BatchAction = {
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

const sampleDeployActionOtherVersion: BatchAction = {
    action: {
        $case: 'deploy',
        deploy: {
            application: 'dummy application',
            version: 30,
            environment: 'dummy environment',
            ignoreAllLocks: false,
            lockBehavior: LockBehavior.Ignore,
        },
    },
};

const sampleDeployActionOtherApplication: BatchAction = {
    action: {
        $case: 'deploy',
        deploy: {
            application: 'dummy application two',
            version: 1,
            environment: 'dummy environment',
            ignoreAllLocks: false,
            lockBehavior: LockBehavior.Ignore,
        },
    },
};

const sampleUndeployAction: BatchAction = {
    action: {
        $case: 'undeploy',
        undeploy: {
            application: 'dummy application',
        },
    },
};

const sampleCreateEnvLock: BatchAction = {
    action: {
        $case: 'createEnvironmentLock',
        createEnvironmentLock: {
            environment: 'dummy environment',
            lockId: '1234',
            message: 'hello',
        },
    },
};

const sampleCreateEnvLockOtherId: BatchAction = {
    action: {
        $case: 'createEnvironmentLock',
        createEnvironmentLock: {
            environment: 'dummy environment',
            lockId: 'newid',
            message: 'hello',
        },
    },
};

const sampleDeleteEnvLock: BatchAction = {
    action: {
        $case: 'deleteEnvironmentLock',
        deleteEnvironmentLock: {
            environment: 'dummy environment',
            lockId: '1234',
        },
    },
};

const sampleCreateAppLock: BatchAction = {
    action: {
        $case: 'createEnvironmentApplicationLock',
        createEnvironmentApplicationLock: {
            application: 'dummy application',
            environment: 'dummy environment',
            lockId: '1111',
            message: 'hi',
        },
    },
};

const sampleCreateAppLockOtherId: BatchAction = {
    action: {
        $case: 'createEnvironmentApplicationLock',
        createEnvironmentApplicationLock: {
            application: 'dummy application',
            environment: 'dummy environment',
            lockId: 'newid',
            message: 'hi',
        },
    },
};

const sampleDeleteAppLock: BatchAction = {
    action: {
        $case: 'deleteEnvironmentApplicationLock',
        deleteEnvironmentApplicationLock: {
            application: 'dummy application',
            environment: 'dummy environment',
            lockId: '1111',
        },
    },
};

const ChildButton = (props: { inCart?: boolean; addToCart?: () => void }) => {
    const { addToCart, inCart } = props;
    return (
        <Button id={'dummy-button'} onClick={addToCart} disabled={inCart}>
            ClickMe
        </Button>
    );
};

describe('Confirmation Dialog Provider', () => {
    const getNode = (overrides?: Partial<ConfirmationDialogProviderProps>, presetActions?: BatchAction[]) => {
        const defaultProps: ConfirmationDialogProviderProps = {
            children: <ChildButton />,
            action: {},
        };
        const value = { actions: presetActions ?? [], setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <ConfirmationDialogProvider {...defaultProps} {...overrides} />;
            </ActionsCartContext.Provider>
        );
    };

    const getWrapper = (overrides?: Partial<ConfirmationDialogProviderProps>, presetActions?: BatchAction[]) =>
        render(getNode({ ...overrides }, presetActions));

    interface dataT {
        type: string;
        act: BatchAction;
        fin?: () => void;
        locks?: [string, Lock][];
        expect: {
            conflict: boolean;
            title: string;
        };
    }

    const data: dataT[] = [
        {
            type: 'Deploy',
            act: sampleDeployAction,
            expect: {
                conflict: true,
                title: 'Are you sure you want to deploy this version?',
            },
        },
        {
            type: 'Un Deploy',
            act: sampleUndeployAction,
            expect: {
                conflict: true,
                title: 'Are you sure you want to undeploy this application?',
            },
        },
        {
            type: 'Create Environment Lock',
            act: sampleCreateEnvLock,
            expect: {
                conflict: true,
                title: 'Are you sure you want to add this environment lock?',
            },
        },
        {
            type: 'Delete Environment Lock',
            act: sampleDeleteEnvLock,
            expect: {
                conflict: false,
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
        {
            type: 'Create Environment Application Lock',
            act: sampleCreateAppLock,
            expect: {
                conflict: true,
                title: 'Are you sure you want to add this application lock?',
            },
        },
        {
            type: 'Delete Environment Application Lock',
            act: sampleDeleteAppLock,
            expect: {
                conflict: false,
                title: 'Are you sure you want to delete this application lock?',
            },
        },
        {
            type: 'With finally function',
            act: sampleDeleteEnvLock,
            fin: finallySpy,
            expect: {
                conflict: false,
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
        {
            type: 'Deploy Action With Locks Warning',
            act: sampleDeployActionOtherApplication,
            locks: [['id_1234', { message: 'random lock message' }]],
            expect: {
                conflict: true,
                title: 'Are you sure you want to deploy this version?',
            },
        },
        {
            type: 'Deploy Action With No Locks Warning',
            act: sampleDeployActionOtherApplication,
            locks: [],
            expect: {
                conflict: false,
                title: 'Are you sure you want to deploy this version?',
            },
        },
    ];

    const sampleCartActions: BatchAction[] = [
        sampleDeployActionOtherVersion,
        sampleCreateEnvLockOtherId,
        sampleCreateAppLockOtherId,
    ];

    describe.each(data)(`Batch Action Types`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            const { container } = getWrapper(
                { action: testcase.act, fin: testcase.fin, locks: testcase.locks },
                sampleCartActions
            );

            // when - clicking the button
            expect(container.querySelector('#dummy-button')!.textContent).toBe('ClickMe');
            fireEvent.click(container.querySelector('#dummy-button')!);

            // test for conflicts
            // when adding the same action should be flagged as conflict
            const res = exportedForTesting.isConflictingAction(sampleCartActions, testcase.act);
            // then ( this function only checks for conflicting actions. checking for locks is not part of it )
            if (!testcase.locks) expect(res).toBe(testcase.expect.conflict);

            if (testcase.expect.conflict) {
                // then a dialog shows up
                const title = document.querySelector('.confirmation-title');
                expect(title!.textContent).toBe(testcase.expect.title);

                // when - clicking yes
                const d = document.querySelector('.MuiDialog-root');
                fireEvent.click(getByText(d! as HTMLElement, 'Add anyway').closest('button')!);

                // then
                mock_setActions.wasCalledWith([...sampleCartActions, testcase.act], Spy.IGNORE);
            } else {
                // then no dialog will show up
                expect(document.querySelector('.confirmation-title')).not.toBeTruthy();
                mock_setActions.wasCalledWith([...sampleCartActions, testcase.act], Spy.IGNORE);
            }
            if (testcase.locks) {
                if (testcase.locks.length > 0) {
                    // when there are locks the warning should appear
                    const d = document.querySelector('.MuiAlert-message');
                    expect(d?.textContent).toContain(testcase.locks[0][0]);
                } else {
                    // when there are no locks the warning should not appear
                    const d = document.querySelector('.MuiAlert-message');
                    expect(d?.textContent).not.toBeTruthy();
                }
            }
            if (testcase.fin) {
                // when a finally function is provided
                finallySpy.wasCalled();
            } else {
                // when a finally function is not provided
                finallySpy.wasNotCalled();
            }
        });
    });

    describe('Action Is Disabled', () => {
        it("disables a button if it's in the cart already", () => {
            // given
            const { container } = getWrapper({ action: data[0].act }, [data[0].act]);

            // when - open the confirmation dialog
            const b = container.querySelector('#dummy-button');
            expect(b!.textContent).toBe('ClickMe');
            expect(b).toBeDisabled();
        });
    });
});
