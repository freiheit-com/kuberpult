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
};

/**
 * A dialog that is used to confirm a question with either yes or no.
 */
export const ConfirmationDialog: React.FC<ConfirmationDialogProps> = (props) => {
    if (!props.open) {
        return <div className={'confirmation-dialog-hidden'}></div>;
    }
    return (
        <div className={'confirmation-dialog-open'}>
            <div className={'confirmation-dialog-header'}>{props.headerLabel}</div>
            <hr />
            <div className={'confirmation-dialog-content'}>{props.children}</div>
            <hr />
            <div className={'confirmation-dialog-footer'}>
                <div className={'item'} key={'button-menu-cancel'}>
                    <Button className="mdc-button--ripple button-cancel" label={'Cancel'} onClick={props.onCancel} />
                </div>
                <div className={'item'} key={'button-menu-confirm'}>
                    <Button
                        className="mdc-button--unelevated button-confirm"
                        label={props.confirmLabel}
                        onClick={props.onConfirm}
                    />
                </div>
            </div>
        </div>
    );
};
