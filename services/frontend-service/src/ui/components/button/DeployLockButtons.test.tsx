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
import { DeployLockButtons, DeployLockButtonsProps } from './DeployLockButtons';

describe('DeployLockButtons', () => {
    const mySubmitSpy = jest.fn(() => {});
    const onClickLockMock = jest.fn(() => {});
    const defaultProps: DeployLockButtonsProps = {
        onClickSubmit: mySubmitSpy,
        onClickLock: onClickLockMock,
        disabled: false,
        releaseDifference: 0,
        deployAlreadyPlanned: false,
        lockAlreadyPlanned: false,
        hasLocks: false,
        unlockAlreadyPlanned: false,
    };

    const getNode = (props: Partial<DeployLockButtonsProps>): JSX.Element => (
        <DeployLockButtons {...Object.assign({}, defaultProps, props)} />
    );

    const getWrapper = (props: Partial<DeployLockButtonsProps>) => render(getNode(props));

    type TestData = {
        name: string;
        props: Partial<DeployLockButtonsProps>;
        // if we click these buttons...
        clickThis: string[];
        // then we expect the popup to show up:
        expectSubmitCalledTimes: number;
        expectSubmitCalledWith: Object; // only relevant if expectCalledTimes != 0
        expectLockCalledTimes: number;
        expectedLabels: string[];
    };

    const data: TestData[] = [
        {
            name: 'click deploy button without release difference',
            props: {},
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock Only', 'Deploy and Lock'],
        },
        {
            name: 'click deploy button when a lock is already planned',
            props: { lockAlreadyPlanned: true },
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Planned Lock', 'Deploy'],
        },
        {
            name: 'click deploy, with positive release difference',
            props: { releaseDifference: 1 },
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock Only', 'Rollback and Lock'],
        },
        {
            name: 'click rollback, with a lock already planned',
            props: { releaseDifference: 1, lockAlreadyPlanned: true },
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Planned Lock', 'Rollback'],
        },
        {
            name: 'click deploy, with negative release difference',
            props: { releaseDifference: -1 },
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock Only', 'Update and Lock'],
        },
        {
            name: 'click update, with a lock already planned',
            props: { releaseDifference: -1, lockAlreadyPlanned: true },
            clickThis: ['.button-main'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Cancel Planned Lock', 'Update'],
        },
        {
            name: 'click lock button',
            props: {},
            clickThis: ['.button-popup-lock'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: true,
            expectLockCalledTimes: 1,
            expectedLabels: ['Add Lock Only', 'Deploy and Lock'],
        },
        {
            name: 'click cancel deploy',
            props: { deployAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock', 'Cancel Deploy'],
        },
        {
            name: 'click cancel rollback',
            props: { releaseDifference: 1, deployAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock', 'Cancel Rollback'],
        },
        {
            name: 'click cancel update',
            props: { releaseDifference: -1, deployAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 1,
            expectSubmitCalledWith: false,
            expectLockCalledTimes: 0,
            expectedLabels: ['Add Lock', 'Cancel Update'],
        },
        {
            name: 'click cancel planned lock',
            props: { lockAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 1,
            expectedLabels: ['Deploy', 'Cancel Planned Lock'],
        },
        {
            name: 'click remove locks',
            props: { hasLocks: true },
            clickThis: ['.button-popup-lock'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 1,
            expectedLabels: ['Remove Locks'],
        },
        {
            name: 'click keep locks',
            props: { hasLocks: true, unlockAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 1,
            expectedLabels: ['Keep Locks'],
        },
        {
            name: 'click cancel lock, with already existing locks',
            props: { hasLocks: true, lockAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 1,
            expectedLabels: ['Cancel Planned Lock'],
        },
        {
            name: 'click cancel lock, with planned removal of existing locks',
            props: { hasLocks: true, unlockAlreadyPlanned: true, lockAlreadyPlanned: true },
            clickThis: ['.deploy-button-cancel'],
            expectSubmitCalledTimes: 0,
            expectSubmitCalledWith: {},
            expectLockCalledTimes: 1,
            expectedLabels: ['Cancel Planned Lock'],
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

            const buttons = Array.from(document.getElementsByClassName('mdc-button__label'));
            const labels = buttons.map((button) => button.textContent);
            expect(labels).toEqual(expect.arrayContaining(testcase.expectedLabels));

            expect(mySubmitSpy).toHaveBeenCalledTimes(testcase.expectSubmitCalledTimes);
            if (testcase.expectSubmitCalledTimes !== 0) {
                expect(mySubmitSpy).toHaveBeenCalledWith(testcase.expectSubmitCalledWith);
            }
            expect(onClickLockMock).toHaveBeenCalledTimes(testcase.expectLockCalledTimes);
        });
    });
});
