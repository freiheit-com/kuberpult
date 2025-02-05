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
        deployAlreadyPlanned: false,
        lockAlreadyPlanned: false,
        hasLocks: false,
        unlockAlreadyPlanned: false,
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
        expectedLabels: string[];
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
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click expand twice',
            props: {},
            clickThis: ['.button-expand', '.button-expand'],
            expectExpanded: false,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click Main button',
            props: {},
            clickThis: ['.button-main'],
            expectExpanded: false,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click expand, then alternative button',
            props: {},
            clickThis: ['.button-expand', '.button-popup-deploy'],
            expectExpanded: true,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click expand, then lock button',
            props: {},
            clickThis: ['.button-expand', '.button-popup-lock'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 1,
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click expand once, with positive release difference',
            props: { releaseDifference: 1 },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Rollback only'],
        },
        {
            name: 'click expand once, with positive release difference',
            props: { releaseDifference: -1 },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Update only'],
        },
        {
            name: 'click expand once, with positive release difference and planned rollback',
            props: { releaseDifference: 1, deployAlreadyPlanned: true, lockAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Rollback only', 'Cancel Lock only'],
        },
        {
            name: 'click expand once, with positive release difference and planned update',
            props: { releaseDifference: -1, deployAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Update only'],
        },
        {
            name: 'click cancel deployment button',
            props: { deployAlreadyPlanned: true, lockAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectExpanded: false,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabels: ['Deploy only'],
        },
        {
            name: 'click expand, then cancel deploy button',
            props: { deployAlreadyPlanned: true },
            clickThis: ['.button-expand', '.button-popup-deploy.deploy-button-cancel'],
            expectExpanded: true,
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Deploy only', 'Lock only'],
        },
        {
            name: 'click expand once, with a lock already planned but no deploy',
            props: { lockAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Deploy only', 'Cancel Lock only'],
        },
        {
            name: 'click expand once, with a deploy already planned but no lock',
            props: { deployAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Deploy only', 'Lock only'],
        },
        {
            name: 'click expand once, with already existing locks',
            props: { hasLocks: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Remove locks'],
        },
        {
            name: 'click expand once, with already existing locks and planned removal',
            props: { hasLocks: true, unlockAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Keep locks'],
        },
        {
            name: 'click expand once, with already existing locks and new lock planned',
            props: { hasLocks: true, lockAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Lock only'],
        },
        {
            name: 'click expand once, with already existing locks and new lock planned and planned removal of existing',
            props: { hasLocks: true, unlockAlreadyPlanned: true, lockAlreadyPlanned: true },
            clickThis: ['.button-expand'],
            expectExpanded: true,
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Lock only'],
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
                expect(labels).toEqual(expect.arrayContaining(testcase.expectedLabels));
            }

            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectSubmitCalledTimes);
            if (testcase.expectSubmitCalledTimes !== 0) {
                expect(mySubmitSpy).toHaveBeenCalledWith(testcase.expectSubmitCalledWith);
            }
            expect(onClickLockMock).toHaveBeenCalledTimes(testcase.expectLockCalledTimes);
        });
    });
});
