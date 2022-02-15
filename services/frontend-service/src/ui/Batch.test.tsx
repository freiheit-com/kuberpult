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
import { ConfirmationDialogProvider, ConfirmationDialogProviderProps } from './Batch';
import { Button } from '@material-ui/core';
import { Spy } from 'spy4js';
import { BatchAction, LockBehavior } from '../api/api';
import { ActionsCartContext } from './App';

const ChildButton = (props: { state?: string; openDialog?: () => void }) => {
    const { openDialog } = props;
    return (
        <Button id={'dialog-opener'} onClick={openDialog}>
            ClickMe
        </Button>
    );
};

const mock_setActions = Spy('setActions');
const finallySpy = Spy('.finally');

describe('Confirmation Dialog Provider', () => {
    const getNode = (overrides?: Partial<ConfirmationDialogProviderProps>) => {
        const defaultProps: ConfirmationDialogProviderProps = {
            children: <ChildButton />,
            action: {},
        };
        const value = { actions: [], setActions: mock_setActions };
        return (
            <ActionsCartContext.Provider value={value}>
                <ConfirmationDialogProvider {...defaultProps} {...overrides} />;
            </ActionsCartContext.Provider>
        );
    };

    const getWrapper = (overrides?: Partial<ConfirmationDialogProviderProps>) => render(getNode({ ...overrides }));

    interface dataT {
        type: string;
        act: BatchAction;
        fin?: () => void;
        expect: {
            title: string;
        };
    }

    const data: dataT[] = [
        {
            type: 'Deploy',
            act: {
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
            expect: {
                title: 'Are you sure you want to deploy this version?',
            },
        },
        {
            type: 'Un Deploy',
            act: {
                action: {
                    $case: 'undeploy',
                    undeploy: {
                        application: 'dummy application',
                    },
                },
            },
            expect: {
                title: 'Are you sure you want to undeploy this application?',
            },
        },
        {
            type: 'Create Environment Lock',
            act: {
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: {
                        environment: 'dummy environment',
                        lockId: '1234',
                        message: 'hello',
                    },
                },
            },
            expect: {
                title: 'Are you sure you want to add this environment lock?',
            },
        },
        {
            type: 'Delete Environment Lock',
            act: {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: 'dummy environment',
                        lockId: '1234',
                    },
                },
            },
            expect: {
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
        {
            type: 'Create Environment Application Lock',
            act: {
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
            expect: {
                title: 'Are you sure you want to add this application lock?',
            },
        },
        {
            type: 'Delete Environment Application Lock',
            act: {
                action: {
                    $case: 'deleteEnvironmentApplicationLock',
                    deleteEnvironmentApplicationLock: {
                        application: 'dummy application',
                        environment: 'dummy environment',
                        lockId: '1111',
                    },
                },
            },
            expect: {
                title: 'Are you sure you want to delete this application lock?',
            },
        },
        {
            type: 'With finally function',
            act: {
                action: {
                    $case: 'deleteEnvironmentLock',
                    deleteEnvironmentLock: {
                        environment: 'dummy environment',
                        lockId: '1234',
                    },
                },
            },
            fin: finallySpy,
            expect: {
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
    ];

    describe.each(data)(`Batch Action Types`, (testcase: dataT) => {
        it(`${testcase.type}`, () => {
            // given
            const { container } = getWrapper({ action: testcase.act, fin: testcase.fin });

            // when - open the confirmation dialog
            expect(container.querySelector('#dialog-opener')!.textContent).toBe('ClickMe');
            fireEvent.click(container.querySelector('#dialog-opener')!);

            // then
            const title = document.querySelector('.confirmation-title');
            expect(title!.textContent).toBe(testcase.expect.title);

            // when - clicking yes
            const d = document.querySelector('.MuiDialog-root');
            fireEvent.click(getByText(d! as HTMLElement, 'Yes').closest('button')!);

            // then
            mock_setActions.wasCalledWith([testcase.act], Spy.IGNORE);

            if (testcase.fin) {
                // when a finally function is provided
                finallySpy.wasCalled();
            } else {
                // when a finally function is not provided
                finallySpy.wasNotCalled();
            }
        });
    });
});
