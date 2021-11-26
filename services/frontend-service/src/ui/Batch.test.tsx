import React from 'react';
import { fireEvent, getByText, render } from '@testing-library/react';
import { ConfirmationDialogProvider, ConfirmationDialogProviderProps, useBatch } from './Batch';
import { Button } from '@material-ui/core';
import { Spy } from 'spy4js';
import { BatchAction, LockBehavior } from '../api/api';

const mock_useBatch = Spy.mockModule('./Batch', 'useBatch');

const fin = Spy('finally');

const ChildButton = (props: {
    state?: string;
    openDialog?: () => void;
}) => {
    const { state, openDialog } = props;
    if (state === 'waiting') {
        return <Button id={'dialog-opener'} onClick={openDialog}>Waiting</Button>
    }else {
        return <Button id={'dialog-opener'} onClick={openDialog}>Hello</Button>
    }
}

describe('Confirmation Dialog Provider', () => {
    const getNode = (overrides?: Partial<ConfirmationDialogProviderProps>) => {
        const defaultProps: ConfirmationDialogProviderProps = {
            children: <ChildButton/>,
            action: {},
        };
        return (
            <ConfirmationDialogProvider {...defaultProps} {...overrides} />
        );
    };

    const getWrapper = (overrides?: Partial<ConfirmationDialogProviderProps>) => render(getNode({...overrides}));

    interface dataT {
        type: string;
        act: BatchAction;
        fin?: () => void;
        expect: {
            title: string;
        }
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
            fin: fin,
            expect: {
                title: 'Are you sure you want to delete this environment lock?',
            },
        },
    ];

    describe.each(data)(`Batch Action Types`, (testcase) => {
        it(`${testcase.type}`,  () => {
            mock_useBatch.useBatch.returns([() => {}, { state: 'resolved' }]);

            const { container } = getWrapper({action: testcase.act, fin: testcase.fin});
            expect(container.querySelector('#dialog-opener')).toBeTruthy();
            fireEvent.click(container.querySelector('#dialog-opener')!);

            console.log (useBatch(testcase.act, testcase.fin));

            const title = document.querySelector('.confirmation-title');
            expect(title).toBeTruthy();
            expect(title!.textContent).toBe(testcase.expect.title);

            const d = document.querySelector('.MuiDialog-root');
            fireEvent.click(getByText(d! as HTMLElement ,'Yes').closest('button')!);

            mock_useBatch.useBatch.wasCalledWith(testcase.act, testcase.fin);
        });
    });
});
