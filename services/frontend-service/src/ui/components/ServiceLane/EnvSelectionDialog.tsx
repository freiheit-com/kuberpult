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

export type EnvSelectionDialogProps = {
    environments: string[];
    onSubmit: (selectedEnvs: string[]) => void;
    onCancel: () => void;
    open: boolean;
    envSelectionDialog: boolean; // false if release train dialog
};

export const EnvSelectionDialog: React.FC<EnvSelectionDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);

    const onConfirm = React.useCallback(() => {
        if (selectedEnvs.length < 1) {
            showSnackbarError('There needs to be at least one environment selected to perform this action');
            return;
        }
        props.onSubmit(selectedEnvs);
        setSelectedEnvs([]);
    }, [props, selectedEnvs]);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedEnvs([]);
    }, [props]);

    const addTeam = React.useCallback(
        (env: string) => {
            const newEnv = env;
            const indexOf = selectedEnvs.indexOf(newEnv);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
            } else if (!props.envSelectionDialog) {
                setSelectedEnvs([newEnv]);
            } else {
                setSelectedEnvs(selectedEnvs.concat(newEnv));
            }
        },
        [props.envSelectionDialog, selectedEnvs]
    );

    return (
        <ConfirmationDialog
            classNames={'env-selection-dialog'}
            onConfirm={onConfirm}
            onCancel={onCancel}
            open={props.open}
            headerLabel={
                props.envSelectionDialog
                    ? 'Select all environments to be removed:'
                    : 'Select which environments to run release train to:'
            }
            confirmLabel={props.envSelectionDialog ? 'Remove app from environments' : 'Release Train'}>
            {props.environments.length > 0 ? (
                <div className="envs-dropdown-select">
                    {props.environments.map((env: string, index: number) => {
                        const enabled = selectedEnvs.includes(env);
                        return (
                            <div key={env}>
                                <Checkbox
                                    enabled={enabled}
                                    onClick={addTeam}
                                    id={String(env)}
                                    label={env}
                                    classes={'env' + env}
                                />
                            </div>
                        );
                    })}
                </div>
            ) : (
                <div className="envs-dropdown-select">
                    {props.envSelectionDialog ? (
                        <div id="missing_envs">There are no environments to list</div>
                    ) : (
                        <div id="missing_envs">
                            There are no available environments to run a release train to based on the current
                            environment/environmentGroup
                        </div>
                    )}
                </div>
            )}
        </ConfirmationDialog>
    );
};
