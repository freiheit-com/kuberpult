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

export type EnvDelDialogProps = {
    onClose: () => void;
    open: boolean;
    envs: Array<string>;
    headerLabel: string;
    confirmLabel: string;
    classNames: string;
    testIdRootRefParent?: string;
};

/**
 * A dialog that is used to confirm a selection.
 */
export const EnvDelDialog: React.FC<EnvDelDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);
    //const { selectedEnvs, setSelectedEnvs } = props;
    const toggleEnv = React.useCallback(
        (env: string) => {
            const newEnv = env;
            const indexOf = selectedEnvs.indexOf(newEnv);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
            } else {
                setSelectedEnvs(selectedEnvs.concat([newEnv]));
            }
        },
        [selectedEnvs, setSelectedEnvs]
    );
    return (
        <PlainDialog
            open={props.open}
            onClose={props.onClose}
            classNames={props.classNames}
            disableBackground={true}
            center={true}
            testIdRootRefParent={props.testIdRootRefParent}
        >
            <>
                <div className={'env-del-dialog-header'}>{props.headerLabel}</div>
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
};
