import { BatchAction } from '../api/api';
import { useUnaryCallback } from './Api';
import * as React from 'react';
import { Dialog, DialogTitle, IconButton } from '@material-ui/core';
import { useCallback } from 'react';
import { DeployButton } from './ReleaseDialog';
import { Close } from '@material-ui/icons';

export const useBatch = (act: BatchAction) =>
    useUnaryCallback(
        React.useCallback(
            (api) =>
                api.batchService().ProcessBatch({
                    actions: [act],
                }),
            [act]
        )
    );

export interface SimpleDialogProps {
    open: boolean;
    onClose: () => void;
    version: number;
    currentlyDeployedVersion: number;
    deployEnv: () => void;
    state: string;
    locked: boolean;
    prefix: string;
    hasQueue: boolean;
}

export const SimpleDialog = (props: SimpleDialogProps) => {
    const { onClose, open } = props;
    const { version, currentlyDeployedVersion, deployEnv, state, locked, prefix, hasQueue } = props;

    const handleClose = useCallback(() => {
        onClose();
    }, [onClose]);

    return (
        <Dialog onClose={handleClose} open={open}>
            <DialogTitle>
                <span>Set message</span>
                <IconButton onClick={handleClose}>
                    <Close />
                </IconButton>
            </DialogTitle>
            <DeployButton
                currentlyDeployedVersion={currentlyDeployedVersion}
                version={version}
                state={state}
                deployEnv={deployEnv}
                locked={locked}
                prefix={prefix}
                hasQueue={hasQueue}
            />
        </Dialog>
    );
};
