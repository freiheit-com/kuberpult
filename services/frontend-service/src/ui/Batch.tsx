import { BatchAction } from '../api/api';
import { useUnaryCallback } from './Api';
import * as React from 'react';
import {
    Button,
    Dialog,
    DialogTitle,
    IconButton,
    Typography,
    Grow,
    ButtonGroup,
    TextField,
    Tooltip,
    CircularProgress,
} from '@material-ui/core';
import { useCallback } from 'react';
import { Close } from '@material-ui/icons';
import AddLockIcon from '@material-ui/icons/EnhancedEncryption';

export const useBatch = (act: BatchAction, fin?: () => void) =>
    useUnaryCallback(
        React.useCallback(
            (api) =>
                api
                    .batchService()
                    .ProcessBatch({
                        actions: [act],
                    })
                    .finally(fin),
            [act, fin]
        )
    );

export interface SimpleDialogProps {
    action: BatchAction;
    currentlyDeployedVersion?: number;
    locked?: boolean;
    hasQueue?: boolean;
    messageBox?: boolean;
    setMessageBox?: (e: boolean) => void;
    message?: string;
    setMessage?: (e: string) => void;
    applicationName?: string;
    fin?: () => void;
}

export const SimpleDialog = (props: SimpleDialogProps) => {
    const { action, currentlyDeployedVersion, locked, hasQueue } = props;
    const { messageBox, setMessageBox, message, setMessage, applicationName, fin } = props;
    const [open, setOpen] = React.useState(false);
    const [doAction, doActionState] = useBatch(action, fin);

    const handleClose = useCallback(() => {
        setOpen(false);
    }, [setOpen]);

    const handleOpen = useCallback(() => {
        setOpen(true);
    }, [setOpen]);

    switch (action.action?.$case) {
        case 'deploy':
            return (
                <>
                    <Button onClick={handleOpen}>Deploy</Button>
                    <Dialog onClose={handleClose} open={open}>
                        <DialogTitle>
                            <Typography variant="subtitle1" component="div" className="confirm">
                                <span>Are you sure you want to deploy this version?</span>
                                <IconButton onClick={handleClose}>
                                    <Close />
                                </IconButton>
                            </Typography>
                        </DialogTitle>
                        <span style={{ alignSelf: 'end' }}>
                            <Button onClick={handleClose}>Cancel</Button>
                            <DeployButton
                                currentlyDeployedVersion={currentlyDeployedVersion!}
                                version={action.action.deploy.version}
                                state={doActionState.state}
                                deployEnv={doAction}
                                locked={locked!}
                                prefix={'deploy '}
                                hasQueue={hasQueue!}
                            />
                        </span>
                    </Dialog>
                </>
            );
        case 'createEnvironmentLock':
        case 'createEnvironmentApplicationLock':
            return (
                <>
                    <Button onClick={handleOpen}>Lock</Button>
                    <Dialog onClose={handleClose} open={open}>
                        <DialogTitle>
                            <Typography variant="subtitle1" component="div" className="confirm">
                                <span>Are you sure you want to add this lock?</span>
                                <IconButton onClick={handleClose}>
                                    <Close />
                                </IconButton>
                            </Typography>
                        </DialogTitle>
                        <span style={{ alignSelf: 'end' }}>
                            <Button onClick={handleClose}>Cancel</Button>
                            <CreateLockButtonButton
                                lock={doAction}
                                state={doActionState.state}
                                open={messageBox!}
                                message={message!}
                                setMessage={setMessage!}
                                setOpen={setMessageBox!}
                                applicationName={applicationName}
                            />
                        </span>
                    </Dialog>
                </>
            );
    }
    return null;
};

const CreateLockButtonButton = (props: {
    applicationName?: string;
    lock: () => void;
    state: string;
    message: string;
    setMessage: (e: string) => void;
    open: boolean;
    setOpen: (e: boolean) => void;
}) => {
    const { applicationName, lock, state, message, setMessage, setOpen, open } = props;

    const updateMessage = React.useCallback((e) => setMessage(e.target.value), [setMessage]);
    const openInput = React.useCallback(() => setOpen(true), [setOpen]);
    switch (state) {
        case 'waiting':
        case 'resolved':
            if (open) {
                return (
                    <Grow in={open} style={{ transformOrigin: 'right center' }}>
                        {applicationName ? (
                            <ButtonGroup className="overlay">
                                <TextField label="Lock Message" variant="standard" onChange={updateMessage} />
                                <IconButton onClick={lock} disabled={message === ''}>
                                    <AddLockIcon />
                                </IconButton>
                            </ButtonGroup>
                        ) : (
                            <ButtonGroup className="overlay">
                                <Button onClick={lock} disabled={message === ''}>
                                    Add Lock
                                </Button>
                                <TextField label="Lock Message" variant="standard" onChange={updateMessage} />
                            </ButtonGroup>
                        )}
                    </Grow>
                );
            } else {
                return applicationName ? (
                    <Tooltip title="Add lock">
                        <IconButton onClick={openInput}>
                            <AddLockIcon />
                        </IconButton>
                    </Tooltip>
                ) : (
                    <Button onClick={openInput}>Add Lock</Button>
                );
            }
        case 'pending':
            return applicationName ? (
                <IconButton disabled>
                    <AddLockIcon />
                </IconButton>
            ) : (
                <Button disabled>Add Lock</Button>
            );
        case 'rejected':
            return applicationName ? <IconButton>Failed</IconButton> : <Button>Failed</Button>;
    }
    return null;
};

const DeployButton = (props: {
    version: number;
    currentlyDeployedVersion: number;
    deployEnv: () => void;
    state: string;
    locked: boolean;
    prefix: string;
    hasQueue: boolean;
}) => {
    const { version, currentlyDeployedVersion, deployEnv, state, locked, prefix, hasQueue } = props;
    const queueMessage = hasQueue ? 'Deploying will also remove the Queue' : '';
    if (version === currentlyDeployedVersion) {
        return (
            <Button variant="contained" disabled>
                {prefix + currentlyDeployedVersion}
            </Button>
        );
    } else {
        switch (state) {
            case 'waiting':
                return (
                    <Tooltip title={queueMessage}>
                        <Button variant="contained" onClick={deployEnv} className={locked ? 'warning' : ''}>
                            {prefix + version}
                        </Button>
                    </Tooltip>
                );
            case 'pending':
                return (
                    <Button variant="contained" disabled>
                        <CircularProgress size={20} />
                    </Button>
                );
            case 'resolved':
                return (
                    <Tooltip title={queueMessage}>
                        <Button variant="contained" disabled>
                            {prefix + currentlyDeployedVersion}
                        </Button>
                    </Tooltip>
                );
            case 'rejected':
                return (
                    <Button variant="contained" disabled>
                        Failed
                    </Button>
                );
            default:
                return null;
        }
    }
};
