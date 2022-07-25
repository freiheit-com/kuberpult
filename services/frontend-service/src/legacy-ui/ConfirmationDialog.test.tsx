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
import { Lock } from '../api/api';
import { ActionsCartContext } from './App';
import { CartAction } from './ActionDetails';

const mock_setActions = Spy('setActions');
const finallySpy = Spy('.finally');

const sampleDeployAction: CartAction = {
    deploy: {
        application: 'dummy application',
        version: 22,
        environment: 'dummy environment',
    },
};

const sampleDeployActionOtherVersion: CartAction = {
    deploy: {
        application: 'dummy application',
        version: 30,
        environment: 'dummy environment',
    },
};

const sampleDeployActionOtherApplication: CartAction = {
    deploy: {
        application: 'dummy application two',
        version: 1,
        environment: 'dummy environment',
    },
};

const sampleUndeployAction: CartAction = {
    undeploy: {
        application: 'dummy application',
    },
};

const sampleCreateEnvLock: CartAction = {
    createEnvironmentLock: {
        environment: 'dummy environment',
    },
};

const sampleCreateEnvLockOtherEnv: CartAction = {
    createEnvironmentLock: {
        environment: 'foo environment',
    },
};

const sampleDeleteEnvLock: CartAction = {
    deleteEnvironmentLock: {
        environment: 'dummy environment',
        lockId: '1234',
    },
};

const sampleCreateAppLock: CartAction = {
    createApplicationLock: {
        application: 'dummy application',
        environment: 'dummy environment',
    },
};

const sampleDeleteAppLock: CartAction = {
    deleteApplicationLock: {
        application: 'dummy application',
        environment: 'dummy environment',
        lockId: '1111',
    },
};

const ChildButton = (props: { inCart?: boolean; addToCart?: () => void }) => {
    const { addToCart, inCart } = props;
    return (
        <Button id={'dummy-button'} onClick={addToCart} disabled={inCart ? true : false}>
            ClickMe
        </Button>
    );
};

describe('Confirmation Dialog Provider', () => {
    const getNode = (overrides?: Partial<ConfirmationDialogProviderProps>, presetActions?: CartAction[]) => {
        const defaultProps: ConfirmationDialogProviderProps = {
            children: <ChildButton />,
            action: sampleCreateEnvLockOtherEnv,
        };
        const value = { actions: presetActions ?? [], setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <ConfirmationDialogProvider {...defaultProps} {...overrides} />;
            </ActionsCartContext.Provider>
        );
    };

    const getWrapper = (overrides?: Partial<ConfirmationDialogProviderProps>, presetActions?: CartAction[]) =>
        render(getNode({ ...overrides }, presetActions));

    interface dataT {
        type: string;
        act: CartAction;
        fin?: () => void;
        undeployedUpstream?: string;
        prefixActions?: CartAction[];
        locks?: [string, Lock][];
        expect: {
            conflict: Set<CartAction>;
            title: string;
        };
    }

    const data: dataT[] = [
        {
            type: 'Deploy',
            act: sampleDeployAction,
            expect: {
                conflict: new Set([sampleDeployActionOtherVersion]),
                title: 'Please be aware:',
            },
        },
        {
            type: 'Un Deploy',
            act: sampleUndeployAction,
            expect: {
                conflict: new Set([sampleDeployActionOtherVersion]),
                title: 'Are you sure you want to undeploy this application?',
            },
        },
        {
            type: 'Create Environment Lock',
            act: sampleCreateEnvLock,
            expect: {
                conflict: new Set(),
                title: 'Are you sure you want to add this environment lock?',
            },
        },
        {
            type: 'Delete Environment Lock',
            act: sampleDeleteEnvLock,
            expect: {
                conflict: new Set(),
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
        {
            type: 'Create Environment Application Lock',
            act: sampleCreateAppLock,
            expect: {
                conflict: new Set(),
                title: 'Are you sure you want to add this application lock?',
            },
        },
        {
            type: 'Delete Environment Application Lock',
            act: sampleDeleteAppLock,
            expect: {
                conflict: new Set(),
                title: 'Are you sure you want to delete this application lock?',
            },
        },
        {
            type: 'With finally function',
            act: sampleDeleteEnvLock,
            fin: finallySpy,
            expect: {
                conflict: new Set(),
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
        {
            type: 'Deploy Action With Locks Warning',
            act: sampleDeployActionOtherApplication,
            locks: [['id_1234', { lockId: 'id_1234', message: 'random lock message' }]],
            expect: {
                conflict: new Set([sampleDeployActionOtherVersion]),
                title: 'Please be aware:',
            },
        },
        {
            type: 'Deploy Action With Undeployed Upstream Warning',
            act: sampleDeployActionOtherApplication,
            undeployedUpstream: 'staging',
            prefixActions: [sampleCreateAppLock],
            expect: {
                conflict: new Set(),
                title: 'Please be aware:',
            },
        },
        {
            type: 'Deploy Action With No Locks Warning',
            act: sampleDeployActionOtherApplication,
            locks: [],
            expect: {
                conflict: new Set(),
                title: 'Please be aware:',
            },
        },
    ];

    const sampleCartActions: CartAction[] = [sampleDeployActionOtherVersion];

    describe.each(data)(`Batch Action Types`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            const { container } = getWrapper(
                {
                    action: testcase.act,
                    fin: testcase.fin,
                    locks: testcase.locks,
                    prefixActions: testcase.prefixActions,
                    undeployedUpstream: testcase.undeployedUpstream,
                },
                sampleCartActions
            );

            // when - clicking the button
            expect(container.querySelector('#dummy-button')!.textContent).toBe('ClickMe');
            fireEvent.click(container.querySelector('#dummy-button')!);

            // test for conflicts
            // when adding the same action should be flagged as conflict
            const res = exportedForTesting.getCartConflicts(sampleCartActions, testcase.act);
            // then ( this function only checks for conflicting actions. checking for locks is not part of it )
            if (!testcase.locks) expect(res).toStrictEqual(testcase.expect.conflict);

            if (testcase.expect.conflict.size || testcase.undeployedUpstream) {
                // then a dialog shows up
                const title = document.querySelector('.confirmation-title');
                expect(title!.textContent).toBe(testcase.expect.title);

                // when - clicking yes
                const d = document.querySelector('.MuiDialog-root');
                fireEvent.click(getByText(d! as HTMLElement, 'Add anyway').closest('button')!);

                // then
                if (testcase.undeployedUpstream) {
                    mock_setActions.wasCalledWith(
                        [...sampleCartActions, ...(testcase.prefixActions || []), testcase.act],
                        Spy.IGNORE
                    );
                } else {
                    mock_setActions.wasCalledWith([...sampleCartActions, testcase.act], Spy.IGNORE);
                }
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

    describe.each([
        { type: 'Sync windows missing entirely', syncWindows: undefined, wantWarning: false },
        { type: 'Without sync windows', syncWindows: [], wantWarning: false },
        {
            type: 'With sync windows',
            syncWindows: [{ kind: 'allow', schedule: '* * * * *', duration: '0s' }],
            wantWarning: true,
        },
    ])(`Confirmation and sync window behaviour`, ({ type, syncWindows, wantWarning }) => {
        it(`${type}`, () => {
            // given
            const { container } = getWrapper({ action: sampleDeployAction, syncWindows }, sampleCartActions);

            // when - clicking the button
            const dummyButton = container.querySelector('#dummy-button');
            if (dummyButton === null) {
                throw new Error(`#dummy-button missing`);
            }

            fireEvent.click(dummyButton);

            // then
            const warning = document.querySelector('.MuiAlert-outlinedWarning');
            if (wantWarning) {
                expect(warning).toBeInTheDocument();
            } else {
                expect(warning).not.toBeInTheDocument();
            }

            const syncWindowElements = document.querySelectorAll('.syncWindow');
            expect(syncWindowElements).toHaveLength(syncWindows?.length ?? 0);
        });
    });
});
