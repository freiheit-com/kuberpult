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

import React, { useState } from 'react';
import { Button } from '../button';
import { PlainDialog } from './ConfirmationDialog';
import { Checkbox } from '../dropdown/checkbox';
import { addAction, deleteAction } from '../../utils/store';
import { BatchAction } from '../../../api/api';

export type EnvDelDialogProps = {
    onClose: () => void;
    open: boolean;
    app: string;
    envs: Array<string>;
    testIdRootRefParent?: string;
};

/**
 * A dialog to remove apps from environments.
 */
export const EnvDelDialog: React.FC<EnvDelDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);
    const toggleEnv = React.useCallback(
        (env: string) => {
            const newEnv = env;
            const action: BatchAction = {
                action: {
                    $case: 'deleteEnvFromApp',
                    deleteEnvFromApp: {
                        environment: newEnv,
                        application: 'echo',
                    },
                },
            };
            const indexOf = selectedEnvs.indexOf(newEnv);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
                deleteAction(action);
            } else {
                setSelectedEnvs(selectedEnvs.concat([newEnv]));
                addAction(action);
            }
        },
        [selectedEnvs, setSelectedEnvs]
    );
    const removeAllEnvs = React.useCallback(() => {
        selectedEnvs.forEach((env) => {
            const action: BatchAction = {
                action: {
                    $case: 'deleteEnvFromApp',
                    deleteEnvFromApp: {
                        environment: env,
                        application: 'echo',
                    },
                },
            };
            deleteAction(action);
        });
        setSelectedEnvs([]);
    }, [selectedEnvs, setSelectedEnvs]);
    return (
        <PlainDialog
            open={props.open}
            onClose={props.onClose}
            classNames="env-del-dialog"
            disableBackground={true}
            center={true}
            testIdRootRefParent={props.testIdRootRefParent}>
            <>
                <div className={'env-del-dialog-header'}>Select the environments to remove for '{props.app}'':</div>
                <hr />
                <div className={'env-del-dialog-content'}>
                    {props.envs.map((env: string) => {
                        const enabled = selectedEnvs.includes(env);
                        return (
                            <div key={env}>
                                <Checkbox
                                    enabled={enabled}
                                    id={String(env)}
                                    label={env}
                                    classes={'env-' + env}
                                    onClick={toggleEnv}
                                />
                            </div>
                        );
                    })}
                </div>
                <hr />
                <div className={'env-del-dialog-footer'}>
                    <div className={'item'} key={'button-remove-all'}>
                        <Button
                            className="mdc-button--unelevated button-confirm test-confirm-button-confirm"
                            testId="test-confirm-button-remove-all"
                            label="Remove app from all remaining environments"
                            onClick={props.onClose}
                            highlightEffect={false}
                        />
                        &nbsp;
                        <Button
                            className="mdc-button--unelevated button-confirm test-confirm-button-confirm"
                            testId="test-confirm-button-remove-none"
                            label="Remove app from no additional environments"
                            onClick={removeAllEnvs}
                            highlightEffect={false}
                        />
                    </div>
                </div>
                <hr />
                <div className={'env-del-dialog-footer'}>
                    <div className={'item'} key={'button-menu-confirm'}>
                        <Button
                            className="mdc-button--unelevated button-confirm test-confirm-button-confirm"
                            testId="test-confirm-button-confirm"
                            label="Finish"
                            onClick={props.onClose}
                            highlightEffect={false}
                        />
                    </div>
                </div>
            </>
        </PlainDialog>
    );
};
