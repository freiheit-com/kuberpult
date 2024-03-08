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

import React from 'react';
import { Button } from '../button';

export type ConfirmationDialogProps = {
    onConfirm: () => void;
    onCancel: () => void;
    open: boolean;
    children: JSX.Element;
    headerLabel: string;
    confirmLabel: string;
    classNames: string;
    testIdRootRefParent?: string;
};

/**
 * A dialog that is used to confirm a question with either yes or no.
 */
export const ConfirmationDialog: React.FC<ConfirmationDialogProps> = (props) => (
    <PlainDialog
        open={props.open}
        onClose={props.onCancel}
        classNames={props.classNames}
        disableBackground={true}
        center={true}
        testIdRootRefParent={props.testIdRootRefParent}>
        <>
            <div className={'confirmation-dialog-header'}>{props.headerLabel}</div>
            <hr />
            <div className={'confirmation-dialog-content'}>{props.children}</div>
            <hr />
            <div className={'confirmation-dialog-footer'}>
                <div className={'item'} key={'button-menu-cancel'} title={'ESC also closes the dialog'}>
                    <Button
                        className="mdc-button--ripple button-cancel"
                        testId="test-button-cancel"
                        label={'Cancel'}
                        onClick={props.onCancel}
                    />
                </div>
                <div className={'item'} key={'button-menu-confirm'}>
                    <Button
                        className="mdc-button--unelevated button-confirm test-button-confirm"
                        testId="test-button-confirm"
                        label={props.confirmLabel}
                        onClick={props.onConfirm}
                    />
                </div>
            </div>
        </>
    </PlainDialog>
);

export type PlainDialogProps = {
    open: boolean;
    onClose: () => void;
    children: JSX.Element;
    classNames: string;
    // NOTE: disableBackground only works for ConfirmationDialog for now, there is no plain-dialog-container-open CSS specification
    disableBackground: boolean;
    center: boolean;
    testIdRootRefParent?: string;
};

/**
 * A dialog that just renders its children. Invoker must take care of all buttons.
 */

export const PlainDialog: React.FC<PlainDialogProps> = (props) => {
    const { onClose, open, children, center, disableBackground, classNames, testIdRootRefParent } = props;
    const classPrefix = center ? 'confirmation' : 'plain';
    const initialRef: HTMLElement | null = null;
    const rootRef = React.useRef(initialRef);

    React.useEffect(() => {
        if (!open) {
            return () => {};
        }
        const winListener = (event: KeyboardEvent): void => {
            if (event.key === 'Escape') {
                onClose();
            }
        };
        const docListener = (event: MouseEvent): void => {
            if (!(event.target instanceof HTMLElement)) {
                return;
            }
            const eventTarget: HTMLElement = event.target;

            if (rootRef.current === null) {
                return;
            }
            const rootRefCurrent: HTMLElement = rootRef.current;

            const isInside = rootRefCurrent.contains(eventTarget);
            if (!isInside) {
                onClose();
            }
        };
        window.addEventListener('keyup', winListener);
        document.addEventListener('pointerup', docListener);
        return () => {
            document.removeEventListener('keyup', winListener);
            document.removeEventListener('pointerup', docListener);
        };
    }, [onClose, open, classPrefix, center]);

    if (!open) {
        return <div className={''}></div>;
    }
    const clas = open && disableBackground ? classPrefix + '-dialog-container-open' : '';
    return (
        <div className={classPrefix + '-dialog-container ' + clas} data-testid={testIdRootRefParent}>
            <div ref={rootRef} className={classPrefix + '-dialog-open ' + (classNames ?? '')}>
                {children}
            </div>
        </div>
    );
};
