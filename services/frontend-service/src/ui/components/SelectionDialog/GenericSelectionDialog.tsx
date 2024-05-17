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
import { Checkbox } from '../dropdown/checkbox';
import { ConfirmationDialog } from '../dialog/ConfirmationDialog';

export type GenericSelectionDialogProps = {
    selectables: string[];
    onSubmit: () => void;
    onCancel: () => void;
    open: boolean;
    multiSelect: boolean;
    headerLabel: string;
    confirmLabel: string;
    onEmptyLabel: string;
    selectedItems: string[];
    setSelectedItems: React.Dispatch<React.SetStateAction<string[]>>;
};

export const GenericSelectionDialog: React.FC<GenericSelectionDialogProps> = (props) => {
    const { selectedSelectables, setSelectedSelectables }  = props;
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
        [props.multiSelect, selectedSelectables, setSelectedSelectables]
    );

    return (
        <ConfirmationDialog
            classNames={'env-selection-dialog'}
            onConfirm={props.onSubmit}
            onCancel={props.onCancel}
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
