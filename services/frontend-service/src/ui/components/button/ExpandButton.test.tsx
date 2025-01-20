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
import React from 'react';
import { act, render } from '@testing-library/react';
import { elementQuerySelectorSafe } from '../../../setupTests';
import { ExpandButton, ExpandButtonProps } from './ExpandButton';

describe('ExpandButton', () => {
    const mySubmitSpy = jest.fn(() => {});
    const onClickLockMock = jest.fn(() => {});
    const defaultProps: ExpandButtonProps = {
        onClickSubmit: mySubmitSpy,
        onClickLock: onClickLockMock,
        disabled: false,
        defaultButtonLabel: 'default-button',
        releaseDifference: 0,
    };

    const getNode = (props: Partial<ExpandButtonProps>): JSX.Element => (
        <ExpandButton {...Object.assign({}, defaultProps, props)} />
    );

    const getWrapper = (props: Partial<ExpandButtonProps>) => render(getNode(props));

    type TestData = {
        name: string;
        props: Partial<ExpandButtonProps>;
        // if we click these buttons...
        clickThis: string[];
        // then we expect the popup to show up:
        expectExpanded: boolean;
        expectSubmitCalledTimes: number;
        expectSubmitCalledWith: Object; // only relevant if expectCalledTimes != 0
        expectLockCalledTimes: number;
        expectedLabel: string;
    };

    const data: TestData[] = [
        {
            name: 'click expand once',
            props: {},
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabel: 'Deploy only',
        },
        {
            name: 'click expand twice',
            props: {},
            clickThis: ['.button-expand', '.button-expand'],
            expectExpanded: false,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabel: 'Deploy only',
        },
        {
            name: 'click Main button',
            props: {},
            clickThis: ['.button-main'],
            expectExpanded: false,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabel: 'Deploy only',
        },
        {
            name: 'click expand, then alternative button',
            props: {},
            clickThis: ['.button-expand', '.button-popup-deploy'],
            expectExpanded: true,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabel: 'Deploy only',
        },
        {
            name: 'click expand, then lock button',
            props: {},
            clickThis: ['.button-expand', '.button-popup-lock'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 1,
            expectedLabel: 'Deploy only',
        },
        {
            name: 'click expand once, with positive release difference',
            props: { releaseDifference: 1 },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabel: 'Rollback only',
        },
        {
            name: 'click expand once, with positive release difference',
            props: { releaseDifference: -1 },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabel: 'Update only',
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            mySubmitSpy.mockReset();
            const { container } = getWrapper(testcase.props);

            expect(document.getElementsByClassName('expand-dialog').length).toBe(0);
            expect(mySubmitSpy).toHaveBeenCalledTimes(0);

            testcase.clickThis.forEach((clickMe: string) => {
                const button = elementQuerySelectorSafe(container, clickMe);
                act(() => {
                    button.click();
                });
            });

            const expectedCount = testcase.expectExpanded ? 1 : 0;
            expect(document.getElementsByClassName('expand-dialog').length).toBe(expectedCount);

            if (expectedCount > 0) {
                const buttons = Array.from(document.getElementsByClassName('mdc-button__label'));
                const labels = buttons.map((button) => button.textContent);
                expect(labels).toEqual(expect.arrayContaining([testcase.expectedLabel]));
            }

            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectSubmitCalledTimes);
            if (testcase.expectSubmitCalledTimes !== 0) {
                expect(mySubmitSpy).toHaveBeenCalledWith(testcase.expectSubmitCalledWith);
            }
            expect(onClickLockMock).toHaveBeenCalledTimes(testcase.expectLockCalledTimes);
        });
    });
});
