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
import * as React from 'react';
import { useState } from 'react';
import { Checkbox } from '../dropdown/checkbox';
import { ConfirmationDialog } from '../dialog/ConfirmationDialog';

export type EnvSelectionDialogProps = {
    environments: string[];
    onSubmit: (selectedEnvs: string[]) => void;
    onCancel: () => void;
    open: boolean;
};

export const EnvSelectionDialog: React.FC<EnvSelectionDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);

    const onConfirm = React.useCallback(() => {
        props.onSubmit(selectedEnvs);
        setSelectedEnvs([]);
    }, [props, selectedEnvs]);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedEnvs([]);
    }, [props]);

    const addTeam = React.useCallback(
        (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {
            const index = Number(e.currentTarget.id);
            const newTeam = props.environments[index];
            const indexOf = selectedEnvs.indexOf(newTeam);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
            } else {
                setSelectedEnvs(selectedEnvs.concat(newTeam));
            }
        },
        [props.environments, selectedEnvs]
    );

    return (
        <ConfirmationDialog
            onConfirm={onConfirm}
            onCancel={onCancel}
            open={props.open}
            headerLabel={'Select all environments to be removed:'}
            confirmLabel={'Remove environments from app'}>
            <div className="envs-dropdown-select">
                {props.environments.map((env: string, index: number) => {
                    const enabled = selectedEnvs.includes(env);
                    return (
                        <div key={env}>
                            <Checkbox
                                enabled={enabled}
                                onClick={addTeam}
                                id={String(index)}
                                label={env}
                                classes={'env' + env}
                            />
                        </div>
                    );
                })}
            </div>
        </ConfirmationDialog>
    );
};
