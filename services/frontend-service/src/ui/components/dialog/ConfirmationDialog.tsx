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
};

/**
 * A dialog that is used to confirm a question with either yes or no.
 */
export const ConfirmationDialog: React.FC<ConfirmationDialogProps> = (props) => (
    <PlainDialog open={props.open} onClose={props.onCancel} classNames={props.classNames}>
        <>
            <div className={'confirmation-dialog-header'}>{props.headerLabel}</div>
            <hr />
            <div className={'confirmation-dialog-content'}>{props.children}</div>
            <hr />
            <div className={'confirmation-dialog-footer'}>
                <div className={'item'} key={'button-menu-cancel'} title={'ESC also closes the dialog'}>
                    <Button
                        className="mdc-button--ripple button-cancel test-button-cancel"
                        label={'Cancel'}
                        onClick={props.onCancel}
                    />
                </div>
                <div className={'item'} key={'button-menu-confirm'}>
                    <Button
                        className="mdc-button--unelevated button-confirm test-button-confirm"
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
};

/**
 * A dialog that just renders its children. Invoker must take care of all buttons.
 */
export const PlainDialog: React.FC<PlainDialogProps> = (props) => {
    const { onClose, open, children } = props;
    React.useEffect(() => {
        window.addEventListener('keyup', (event) => {
            if (event.key === 'Escape') {
                onClose();
            }
        });
        document.addEventListener('click', (event) => {
            if (open) {
                if (event.target instanceof HTMLElement) {
                    const isOutside = event.target.className.indexOf('confirmation-dialog-container') >= 0;
                    if (isOutside) {
                        onClose();
                    }
                }
            }
        });
    }, [onClose, open]);

    if (!open) {
        return <div className={'confirmation-dialog-hidden'}></div>;
    }
    return (
        <div className={'confirmation-dialog-container ' + (props.open ? 'confirmation-dialog-container-open' : '')}>
            <div className={'confirmation-dialog-open release-dialog ' + props.classNames}>{children}</div>
        </div>
    );
};
