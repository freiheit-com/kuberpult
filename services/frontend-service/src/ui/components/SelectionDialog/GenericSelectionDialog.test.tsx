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

Copyright freiheit.com*/
import { act, render, getByTestId } from '@testing-library/react';
import { documentQuerySelectorSafe } from '../../../setupTests';
import { GenericSelectionDialog, GenericSelectionDialogProps } from './GenericSelectionDialog';

type TestDataSelection = {
    name: string;
    input: GenericSelectionDialogProps;
    expectedNumItems: number;
    clickOnButton: string;
    secondClick: string;
    expectedNumSelectedAfterClick: number;
    expectedNumDeselectedAfterClick: number;
    expectedNumSelectedAfterSecondClick: number;
};

const mySubmitSpy = jest.fn();
const myCancelSpy = jest.fn();

const confirmButtonTestId = 'test-confirm-button-confirm';
const cancelButtonTestId = 'test-confirm-button-cancel';
const envSelectionDialogHeaderLabel = 'Select all environments to be removed:';
const envSelectionDialogConfirmLabel = 'Remove app from environments';

const envSelectionDialogonEmptyLabel = 'There are no environments to list';

const dataSelection: TestDataSelection[] = [
    {
        name: 'renders 2 item list',
        input: {
            selectables: ['dev', 'staging'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        expectedNumItems: 2,
        clickOnButton: 'dev',
        expectedNumSelectedAfterClick: 1,
        expectedNumDeselectedAfterClick: 1,
        secondClick: 'staging',
        expectedNumSelectedAfterSecondClick: 2,
    },
    {
        name: 'renders 3 item list',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        expectedNumItems: 3,
        clickOnButton: 'staging',
        expectedNumSelectedAfterClick: 1,
        expectedNumDeselectedAfterClick: 2,
        secondClick: 'prod',
        expectedNumSelectedAfterSecondClick: 2,
    },
    {
        name: 'only one item allowed for release trains',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: false,
            headerLabel: 'Select which environments to run release train to:',
            confirmLabel: 'Release Train',
            onEmptyLabel:
                'There are no available environments to run a release train to based on the current environment/environmentGroup',
        },
        expectedNumItems: 3,
        clickOnButton: 'staging',
        expectedNumSelectedAfterClick: 1,
        expectedNumDeselectedAfterClick: 2,
        secondClick: 'prod',
        expectedNumSelectedAfterSecondClick: 1,
    },
    {
        name: 'renders empty item list',
        input: {
            selectables: [],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        expectedNumItems: 0,
        clickOnButton: '',
        expectedNumSelectedAfterClick: 0,
        expectedNumDeselectedAfterClick: 0,
        secondClick: '',
        expectedNumSelectedAfterSecondClick: 0,
    },
];

type TestDataOpenClose = {
    name: string;
    input: GenericSelectionDialogProps;
    expectedNumElements: number;
};
const dataOpenClose: TestDataOpenClose[] = [
    {
        name: 'renders open dialog',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        expectedNumElements: 1,
    },
    {
        name: 'renders closed dialog',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: false,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        expectedNumElements: 0,
    },
];

type TestDataCallbacks = {
    name: string;
    input: GenericSelectionDialogProps;
    clickThis: string;
    expectedCancelCallCount: number;
    expectedSubmitCallCount: number;
};
const dataCallbacks: TestDataCallbacks[] = [
    {
        name: 'renders open dialog',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        clickThis: cancelButtonTestId,
        expectedCancelCallCount: 1,
        expectedSubmitCallCount: 0,
    },
    {
        name: 'renders closed dialog',
        input: {
            selectables: ['dev', 'staging', 'prod'],
            open: true,
            onSubmit: mySubmitSpy,
            onCancel: myCancelSpy,
            multiSelect: true,
            headerLabel: envSelectionDialogHeaderLabel,
            confirmLabel: envSelectionDialogConfirmLabel,
            onEmptyLabel: envSelectionDialogonEmptyLabel,
        },
        clickThis: confirmButtonTestId,
        expectedCancelCallCount: 0,
        expectedSubmitCallCount: 0,
    },
];

const getNode = (overrides: GenericSelectionDialogProps) => <GenericSelectionDialog {...overrides} />;
const getWrapper = (overrides: GenericSelectionDialogProps) => render(getNode(overrides));

describe('EnvSelectionDialog', () => {
    describe.each(dataSelection)('Test checkbox enabled', (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            myCancelSpy.mockReset();
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            expect(myCancelSpy).toHaveBeenCalledTimes(0);

            getWrapper(testcase.input);

            expect(document.querySelectorAll('.envs-dropdown-select .test-button-checkbox').length).toEqual(
                testcase.expectedNumItems
            );
            if (testcase.clickOnButton !== '') {
                const result = documentQuerySelectorSafe('.id-' + testcase.clickOnButton);
                act(() => {
                    result.click();
                });
            } else {
                expect(document.querySelector('.env-selection-dialog')?.textContent).toContain(
                    envSelectionDialogonEmptyLabel
                );
            }
            expect(document.querySelectorAll('.test-button-checkbox.enabled').length).toEqual(
                testcase.expectedNumSelectedAfterClick
            );
            expect(document.querySelectorAll('.test-button-checkbox.disabled').length).toEqual(
                testcase.expectedNumDeselectedAfterClick
            );
            if (testcase.secondClick !== '') {
                const result = documentQuerySelectorSafe('.id-' + testcase.secondClick);
                act(() => {
                    result.click();
                });
            }
            expect(document.querySelectorAll('.test-button-checkbox.enabled').length).toEqual(
                testcase.expectedNumSelectedAfterSecondClick
            );
        });
    });
    describe.each(dataOpenClose)('open/close', (testcase) => {
        it(testcase.name, () => {
            getWrapper(testcase.input);
            expect(document.querySelectorAll('.envs-dropdown-select').length).toEqual(testcase.expectedNumElements);
        });
    });
    describe.each(dataCallbacks)('submit/cancel callbacks', (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            myCancelSpy.mockReset();
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            expect(myCancelSpy).toHaveBeenCalledTimes(0);

            const { container } = getWrapper(testcase.input);

            const theButton = getByTestId(container, testcase.clickThis);
            act(() => {
                theButton.click();
            });
            getByTestId(container, testcase.clickThis); // should not crash

            expect(myCancelSpy).toHaveBeenCalledTimes(testcase.expectedCancelCallCount);
            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectedSubmitCallCount);
        });
    });

    type TestDataAddTeam = {
        name: string;
        input: GenericSelectionDialogProps;
        clickTheseTeams: string[];
        expectedCancelCallCount: number;
        expectedSubmitCallCount: number;
        expectedSubmitCalledWith: string[];
    };
    const addTeamArray: TestDataAddTeam[] = [
        {
            name: '1 env',
            input: {
                selectables: ['dev', 'staging', 'prod'],
                open: true,
                onSubmit: mySubmitSpy,
                onCancel: myCancelSpy,
                multiSelect: true,
                headerLabel: envSelectionDialogHeaderLabel,
                confirmLabel: envSelectionDialogConfirmLabel,
                onEmptyLabel: envSelectionDialogonEmptyLabel,
            },
            clickTheseTeams: ['dev'],
            expectedCancelCallCount: 0,
            expectedSubmitCallCount: 1,
            expectedSubmitCalledWith: ['dev'],
        },
        {
            name: '2 envs',
            input: {
                selectables: ['dev', 'staging', 'prod'],
                open: true,
                onSubmit: mySubmitSpy,
                onCancel: myCancelSpy,
                multiSelect: true,
                headerLabel: envSelectionDialogHeaderLabel,
                confirmLabel: envSelectionDialogConfirmLabel,
                onEmptyLabel: envSelectionDialogonEmptyLabel,
            },
            clickTheseTeams: ['staging', 'prod'],
            expectedCancelCallCount: 0,
            expectedSubmitCallCount: 1,
            expectedSubmitCalledWith: ['staging', 'prod'],
        },
        {
            name: '1 env clicked twice',
            input: {
                selectables: ['dev', 'staging', 'prod'],
                open: true,
                onSubmit: mySubmitSpy,
                onCancel: myCancelSpy,
                multiSelect: true,
                headerLabel: envSelectionDialogHeaderLabel,
                confirmLabel: envSelectionDialogConfirmLabel,
                onEmptyLabel: envSelectionDialogonEmptyLabel,
            },
            clickTheseTeams: ['dev', 'staging', 'staging'],
            expectedCancelCallCount: 0,
            expectedSubmitCallCount: 1,
            expectedSubmitCalledWith: ['dev'],
        },
    ];
    describe.each(addTeamArray)('adding 2 teams works', (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            myCancelSpy.mockReset();
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            expect(myCancelSpy).toHaveBeenCalledTimes(0);

            const { container } = getWrapper(testcase.input);

            testcase.clickTheseTeams.forEach((value, index) => {
                const teamButton = documentQuerySelectorSafe('.id-' + value);
                act(() => {
                    teamButton.click();
                });
            });
            const confirmButton = getByTestId(container, confirmButtonTestId);
            act(() => {
                confirmButton.click();
            });

            expect(myCancelSpy).toHaveBeenCalledTimes(testcase.expectedCancelCallCount);
            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectedSubmitCallCount);
            expect(mySubmitSpy).toHaveBeenCalledWith(testcase.expectedSubmitCalledWith);
        });
    });
});
