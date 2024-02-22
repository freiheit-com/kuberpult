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
import { useEnvironments } from '../../utils/store';

export type ReleaseTrainDialogProps = {
    environment: string;
    onCancel: () => void;
    open: boolean;
};

export const ReleaseTrainDialog: React.FC<ReleaseTrainDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);

    const envsList = useEnvironments();
    const onConfirm = React.useCallback(() => {
        setSelectedEnvs([]);
    }, []);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedEnvs([]);
    }, [props]);

    const addEnv = React.useCallback(
        (env: string) => {
            const newEnv = env;
            const indexOf = selectedEnvs.indexOf(newEnv);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
            } else {
                setSelectedEnvs(selectedEnvs.concat(newEnv));
            }
        },
        [selectedEnvs]
    );

    return (
        <ConfirmationDialog
            classNames={'env-selection-dialog'}
            onConfirm={onConfirm}
            onCancel={onCancel}
            open={props.open}
            headerLabel={'Select which environments to run release train to:'}
            confirmLabel={'Release Train'}>
            {envsList.filter((env, index) => props.environment === env.config?.upstream?.environment).length > 0 ? (
                <div className="envs-dropdown-select">
                    {envsList
                        .filter((env, index) => props.environment === env.config?.upstream?.environment)
                        .map((env) => {
                            const enabled = selectedEnvs.includes(env.name);
                            return (
                                <div key={env.name}>
                                    <Checkbox
                                        enabled={enabled}
                                        onClick={addEnv}
                                        id={String(env.name)}
                                        label={env.name}
                                        classes={'env' + env.name}
                                    />
                                </div>
                            );
                        })}
                </div>
            ) : (
                <div>There are no environments that have environment {props.environment} as the upstream target</div>
            )}
        </ConfirmationDialog>
    );
};
