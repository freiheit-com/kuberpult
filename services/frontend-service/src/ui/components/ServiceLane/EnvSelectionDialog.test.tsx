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
import { act, render } from '@testing-library/react';
import { documentQuerySelectorSafe } from '../../../setupTests';
import { EnvSelectionDialog, EnvSelectionDialogProps } from './EnvSelectionDialog';

type TestDataSelection = {
    name: string;
    input: EnvSelectionDialogProps;
    expectedNumItems: number;
    clickOnButton: string;
    expectedNumSelectedAfterClick: number;
    expectedNumDeselectedAfterClick: number;
};

const mySubmitSpy = jest.fn();
const myCancelSpy = jest.fn();

const dataSelection: TestDataSelection[] = [
    {
        name: 'renders 2 item list',
        input: { environments: ['dev', 'staging'], open: true, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        expectedNumItems: 2,
        clickOnButton: 'dev',
        expectedNumSelectedAfterClick: 1,
        expectedNumDeselectedAfterClick: 1,
    },
    {
        name: 'renders 3 item list',
        input: { environments: ['dev', 'staging', 'prod'], open: true, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        expectedNumItems: 3,
        clickOnButton: 'staging',
        expectedNumSelectedAfterClick: 1,
        expectedNumDeselectedAfterClick: 2,
    },
];

type TestDataOpenClose = {
    name: string;
    input: EnvSelectionDialogProps;
    expectedNumElements: number;
};
const dataOpenClose: TestDataOpenClose[] = [
    {
        name: 'renders open dialog',
        input: { environments: ['dev', 'staging', 'prod'], open: true, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        expectedNumElements: 1,
    },
    {
        name: 'renders closed dialog',
        input: { environments: ['dev', 'staging', 'prod'], open: false, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        expectedNumElements: 0,
    },
];

type TestDataCallbacks = {
    name: string;
    input: EnvSelectionDialogProps;
    clickThis: string;
    expectedCancelCallCount: number;
    expectedSubmitCallCount: number;
};
const dataCallbacks: TestDataCallbacks[] = [
    {
        name: 'renders open dialog',
        input: { environments: ['dev', 'staging', 'prod'], open: true, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        clickThis: '.test-button-cancel',
        expectedCancelCallCount: 1,
        expectedSubmitCallCount: 0,
    },
    {
        name: 'renders closed dialog',
        input: { environments: ['dev', 'staging', 'prod'], open: true, onSubmit: mySubmitSpy, onCancel: myCancelSpy },
        clickThis: '.test-button-confirm',
        expectedCancelCallCount: 0,
        expectedSubmitCallCount: 1,
    },
];

const getNode = (overrides: EnvSelectionDialogProps) => <EnvSelectionDialog {...overrides} />;
const getWrapper = (overrides: EnvSelectionDialogProps) => render(getNode(overrides));

describe('EnvSelectionDialog Rendering', () => {
    describe.each(dataSelection)('EnvSelectionDialog Test', (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            myCancelSpy.mockReset();
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            expect(myCancelSpy).toHaveBeenCalledTimes(0);

            getWrapper(testcase.input);

            expect(document.querySelectorAll('.envs-dropdown-select .test-button-env-selection').length).toEqual(
                testcase.expectedNumItems
            );
            const result = documentQuerySelectorSafe('.env-' + testcase.clickOnButton);
            act(() => {
                result.click();
            });
            expect(document.querySelectorAll('.test-button-env-selection.enabled').length).toEqual(
                testcase.expectedNumSelectedAfterClick
            );
            expect(document.querySelectorAll('.test-button-env-selection.disabled').length).toEqual(
                testcase.expectedNumDeselectedAfterClick
            );
        });
    });
    describe.each(dataOpenClose)('EnvSelectionDialog open/close', (testcase) => {
        it(testcase.name, () => {
            getWrapper(testcase.input);
            expect(document.querySelectorAll('.envs-dropdown-select').length).toEqual(testcase.expectedNumElements);
        });
    });
    describe.each(dataCallbacks)('EnvSelectionDialog callbacks', (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            myCancelSpy.mockReset();
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);
            expect(myCancelSpy).toHaveBeenCalledTimes(0);

            getWrapper(testcase.input);

            const theButton = documentQuerySelectorSafe(testcase.clickThis);
            act(() => {
                theButton.click();
            });
            documentQuerySelectorSafe(testcase.clickThis); // should not crash

            expect(myCancelSpy).toHaveBeenCalledTimes(testcase.expectedCancelCallCount);
            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectedSubmitCallCount);
        });
    });
});
