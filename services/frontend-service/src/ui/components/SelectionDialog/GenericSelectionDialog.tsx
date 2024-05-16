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
import * as React from 'react';
import { useState } from 'react';
import { Checkbox } from '../dropdown/checkbox';
import { ConfirmationDialog } from '../dialog/ConfirmationDialog';
import { showSnackbarError } from '../../utils/store';

export type GenericSelectionDialogProps = {
    selectables: string[];
    onSubmit: (selected: string[]) => void;
    onCancel: () => void;
    open: boolean;
    multiSelect: boolean; // false if release train dialog
    headerLabel: string;
    confirmLabel: string;
    onEmptyLabel: string;
};

export const GenericSelectionDialog: React.FC<GenericSelectionDialogProps> = (props) => {
    const [selectedSelectables, setSelectedSelectables] = useState<string[]>([]);

    const onConfirm = React.useCallback(() => {
        if (selectedSelectables.length < 1) {
            showSnackbarError('There needs to be at least one team selected to perform this action');
            return;
        }
        props.onSubmit(selectedSelectables);
        setSelectedSelectables([]);
    }, [props, selectedSelectables]);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedSelectables([]);
    }, [props]);

    const addSelectable = React.useCallback(
        (selectable: string) => {
            const newSelectable = selectable;
            const indexOf = selectedSelectables.indexOf(newSelectable);
            if (indexOf >= 0) {
                const copy = selectedSelectables.concat();
                copy.splice(indexOf, 1);
                setSelectedSelectables(copy);
            } else if (!props.multiSelect) {
                setSelectedSelectables([newSelectable]);
            } else {
                setSelectedSelectables(selectedSelectables.concat(newSelectable));
            }
        },
        [props.multiSelect, selectedSelectables]
    );

    return (
        <ConfirmationDialog
            classNames={'env-selection-dialog'}
            onConfirm={onConfirm}
            onCancel={onCancel}
            open={props.open}
            headerLabel={props.headerLabel}
            confirmLabel={props.confirmLabel}>
            {props.selectables.length > 0 ? (
                <div className="envs-dropdown-select">
                    {props.selectables.map((selectable: string) => {
                        const enabled = selectedSelectables.includes(selectable);
                        return (
                            <div key={selectable}>
                                <Checkbox
                                    enabled={enabled}
                                    onClick={addSelectable}
                                    id={String(selectable)}
                                    label={selectable}
                                    classes={'selectable' + selectable}
                                />
                            </div>
                        );
                    })}
                </div>
            ) : (
                <div className="envs-dropdown-select">{<div id="missing_envs">{props.onEmptyLabel}</div>}</div>
            )}
        </ConfirmationDialog>
    );
};
