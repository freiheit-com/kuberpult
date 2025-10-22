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
import { Button } from '../button';
import { PlainDialog } from './ConfirmationDialog';

export type ClosableDialogProps = {
    onClose: () => void;
    open: boolean;
    children: JSX.Element;
    headerLabel: string;
    confirmLabel: string;
    classNames: string;
    testIdRootRefParent?: string;
};

/**
 * A dialog that is used to confirm a selection.
 */
export const ClosableDialog: React.FC<ClosableDialogProps> = (props) => (
    <PlainDialog
        open={props.open}
        onClose={props.onClose}
        classNames={props.classNames}
        disableBackground={true}
        center={true}
        testIdRootRefParent={props.testIdRootRefParent}>
        <>
            <div className={'closeable-dialog-header'}>{props.headerLabel}</div>
            <hr />
            <div className={'closable-dialog-content'}>{props.children}</div>
            <hr />
            <div className={'closeable-dialog-footer'}>
                <div className={'item'} key={'button-menu-confirm'}>
                    <Button
                        className="mdc-button--unelevated button-confirm test-confirm-button-confirm"
                        testId="test-confirm-button-confirm"
                        label={props.confirmLabel}
                        onClick={props.onClose}
                        highlightEffect={false}
                    />
                </div>
            </div>
        </>
    </PlainDialog>
);
