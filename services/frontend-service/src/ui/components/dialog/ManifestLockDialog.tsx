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

import React, { useEffect, useState } from 'react';
import { Button } from '../button';
import { PlainDialog } from './ConfirmationDialog';
import { Checkbox } from '../dropdown/checkbox';
import { addAction, deleteAction, useCreateManifestLockActionsForApp } from '../../utils/store';
import { BatchAction } from '../../../api/api';

export type ManifestLockDialogProps = {
    onClose: () => void;
    open: boolean;
    app: string;
    envs: string[];
    lockedEnvs: string[];
};

const manifestLockAction = (app: string, env: string): BatchAction => ({
    action: {
        $case: 'createManifestLock',
        createManifestLock: {
            app,
            env,
            message: '',
            suggestedLifeTime: undefined,
        },
    },
});

export const ManifestLockDialog: React.FC<ManifestLockDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);
    const existingActions = useCreateManifestLockActionsForApp(props.app);
    useEffect(() => {
        if (props.open) {
            setSelectedEnvs(existingActions);
        }
        return () => {};
    }, [props.open, existingActions]);

    const toggleEnv = React.useCallback(
        (env: string) => {
            const action = manifestLockAction(props.app, env);
            const indexOf = selectedEnvs.indexOf(env);
            if (indexOf >= 0) {
                const copy = selectedEnvs.concat();
                copy.splice(indexOf, 1);
                setSelectedEnvs(copy);
                deleteAction(action);
            } else {
                setSelectedEnvs(selectedEnvs.concat([env]));
                addAction(action);
            }
        },
        [selectedEnvs, setSelectedEnvs, props.app]
    );

    return (
        <PlainDialog
            open={props.open}
            onClose={props.onClose}
            classNames="manifest-lock-dialog"
            disableBackground={true}
            center={true}>
            <>
                <div className={'manifest-lock-dialog-header'}>Create Manifest Lock for &apos;{props.app}&apos;</div>
                <div className={'manifest-lock-dialog-description'}>
                    A manifest lock prevents deployment manifests from being written for the selected app/env
                    combination. Unlike regular locks, manifest locks take effect instantly without going through the
                    deployment queue. <br /> You still need apply the planned actions.
                </div>
                <div>
                    <br />
                </div>
                <div className={'manifest-lock-dialog-description'}>Select environments:</div>
                <hr />
                <div className={'manifest-lock-dialog-content'}>
                    {props.envs.map((env: string) => {
                        const alreadyLocked = props.lockedEnvs.includes(env);
                        const selected = selectedEnvs.includes(env);
                        return (
                            <div key={env} className={alreadyLocked ? 'manifest-lock-env-disabled' : ''}>
                                <Checkbox
                                    enabled={alreadyLocked ? false : selected}
                                    id={env}
                                    label={alreadyLocked ? env + ' (manifest lock exists)' : env}
                                    classes={'env-' + env}
                                    onClick={alreadyLocked ? undefined : toggleEnv}
                                />
                            </div>
                        );
                    })}
                </div>
                <hr />
                <div className={'manifest-lock-dialog-footer'}>
                    <Button
                        className="mdc-button--unelevated button-confirm"
                        label="Close"
                        onClick={props.onClose}
                        highlightEffect={false}
                    />
                </div>
            </>
        </PlainDialog>
    );
};
