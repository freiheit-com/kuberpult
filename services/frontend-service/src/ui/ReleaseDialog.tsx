/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import * as React from 'react';

import { makeStyles } from '@material-ui/core/styles';

import ArrowLeftIcon from '@material-ui/icons/ArrowLeft';
import ArrowRightIcon from '@material-ui/icons/ArrowRight';
import BlockIcon from '@material-ui/icons/Block';
import Button from '@material-ui/core/Button';
import ButtonGroup from '@material-ui/core/ButtonGroup';
import Dialog from '@material-ui/core/Dialog';
import DialogActions from '@material-ui/core/DialogActions';
import DialogContent from '@material-ui/core/DialogContent';
import DialogContentText from '@material-ui/core/DialogContentText';
import DialogTitle from '@material-ui/core/DialogTitle';
import Grid from '@material-ui/core/Grid';
import IconButton from '@material-ui/core/IconButton';
import EqualIcon from '@material-ui/icons/DragHandle';
import Paper from '@material-ui/core/Paper';
import LockIcon from '@material-ui/icons/Lock';
import Tooltip from '@material-ui/core/Tooltip';
import CircularProgress from '@material-ui/core/CircularProgress';
import Typography from '@material-ui/core/Typography';

import { useUnaryCallback } from './Api';

import type { GetOverviewResponse, Application, Lock, BatchAction } from '../api/api';
import { LockBehavior } from '../api/api';
import { EnvSortOrder, sortEnvironmentsByUpstream } from './Releases';
import { ConfirmationDialogProvider } from './Batch';
import { useCallback, useMemo } from 'react';
import { Grow, TextField } from '@material-ui/core';
import AddLockIcon from '@material-ui/icons/EnhancedEncryption';

type Data = { applicationName: string; version: number };
export const Context = React.createContext<{ setData: (d: Data | null) => void }>({
    setData: () => {
        throw new Error('No release dialog provider set');
    },
});

const VersionDiff = (props: { current: number | undefined; target: number }) => {
    const { current, target } = props;
    const prefix = 'currently deployed: ';
    if (current === undefined) {
        return (
            <Tooltip title={prefix + 'not deployed'}>
                <BlockIcon className="notDeployed" />
            </Tooltip>
        );
    }
    if (current > target) {
        return (
            <Tooltip title={prefix + '' + (current - target) + ' ahead'}>
                <span className="ahead">{'+' + (current - target)}</span>
            </Tooltip>
        );
    } else if (current < target) {
        return (
            <Tooltip title={prefix + (target - current) + ' behind'}>
                <span className="behind">{'-' + (target - current)}</span>
            </Tooltip>
        );
    } else {
        return (
            <Tooltip title="same version">
                <EqualIcon className="same" />
            </Tooltip>
        );
    }
};

const QueueDiff = (props: { queued: number; current: number }) => {
    const prefix = 'queued: ';
    const { queued, current } = props;
    if (queued === 0) {
        // no queue
        return (
            <Tooltip title="nothing queued">
                <span>
                    <BlockIcon className="notDeployed" />
                </span>
            </Tooltip>
        );
    }
    const diff = queued - current;
    if (diff === 0) {
        return (
            <Tooltip title={prefix + 'same version'}>
                <EqualIcon className="same" />
            </Tooltip>
        );
    }
    if (diff > 0) {
        return (
            <Tooltip title={prefix + diff + ' ahead'}>
                <span>
                    &nbsp;
                    {' queued: '}
                    <span className="ahead">{'+' + diff}</span>
                </span>
            </Tooltip>
        );
    }
    return (
        <Tooltip title={prefix + diff + ' behind'}>
            <span>
                <span className="ahead">{'+' + diff}</span>
            </span>
        </Tooltip>
    );
};

export const randomLockId = () => 'ui-' + Math.random().toString(36).substring(7);

const LockButtonGroup = (props: {
    applicationName?: string;
    openDialog?: () => void;
    state?: string;
    message: string;
    setMessage: (e: string) => void;
    open: boolean;
    setOpen: (e: boolean) => void;
}) => {
    const { applicationName, openDialog, state, message, setMessage, setOpen, open } = props;
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
                                <IconButton onClick={openDialog} disabled={message === ''}>
                                    <AddLockIcon />
                                </IconButton>
                            </ButtonGroup>
                        ) : (
                            <ButtonGroup className="overlay">
                                <Button onClick={openDialog} disabled={message === ''}>
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

const ReleaseLockButtonGroup = (props: {
    lock: Lock;
    state?: string;
    openDialog?: () => void;
    queueHint?: boolean;
}) => {
    const { lock, queueHint, state, openDialog } = props;

    const msg = queueHint ? 'When you unlock the last lock the queue will be deployed!' : '';
    switch (state) {
        case 'waiting':
            return (
                <Tooltip arrow title={'Locked with message "' + lock.message + '". Click to unlock. ' + msg}>
                    <IconButton onClick={openDialog}>
                        <LockIcon />
                    </IconButton>
                </Tooltip>
            );
        case 'pending':
            return (
                <IconButton disabled>
                    <CircularProgress size={20} />
                </IconButton>
            );
        default:
            return null;
    }
};

const DeployButton = (props: {
    version: number;
    currentlyDeployedVersion: number;
    openDialog?: () => void;
    state?: string;
    locked: boolean;
    prefix: string;
    hasQueue: boolean;
}) => {
    const { version, currentlyDeployedVersion, openDialog, state, locked, prefix, hasQueue } = props;
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
                        <Button variant="contained" onClick={openDialog} className={locked ? 'warning' : ''}>
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

export const CreateLockButton = (props: { applicationName?: string; environmentName: string }) => {
    const { applicationName, environmentName } = props;
    const [messageBox, setMessageBox] = React.useState(false);
    const [message, setMessage] = React.useState('');
    const act: BatchAction = useMemo(
        () => ({
            action: applicationName
                ? {
                      $case: 'createEnvironmentApplicationLock',
                      createEnvironmentApplicationLock: {
                          application: applicationName,
                          environment: environmentName,
                          lockId: randomLockId(),
                          message: message,
                      },
                  }
                : {
                      $case: 'createEnvironmentLock',
                      createEnvironmentLock: {
                          environment: environmentName,
                          lockId: randomLockId(),
                          message: message,
                      },
                  },
        }),
        [applicationName, environmentName, message]
    );

    const fin = useCallback(() => {
        setMessageBox(false);
    }, [setMessageBox]);

    return (
        <ConfirmationDialogProvider action={act} fin={fin}>
            <LockButtonGroup
                open={messageBox}
                message={message}
                setMessage={setMessage}
                setOpen={setMessageBox}
                applicationName={applicationName}
            />
        </ConfirmationDialogProvider>
    );
};

export const ReleaseLockButton = (props: {
    applicationName?: string;
    environmentName: string;
    lockId: string;
    lock: Lock;
    queueHint?: boolean;
}) => {
    const { applicationName, environmentName, lock, lockId, queueHint } = props;

    const act: BatchAction = useMemo(
        () => ({
            action: applicationName
                ? {
                      $case: 'deleteEnvironmentApplicationLock',
                      deleteEnvironmentApplicationLock: {
                          application: applicationName,
                          environment: environmentName,
                          lockId: lockId,
                      },
                  }
                : {
                      $case: 'deleteEnvironmentLock',
                      deleteEnvironmentLock: {
                          environment: environmentName,
                          lockId: lockId,
                      },
                  },
        }),
        [applicationName, environmentName, lockId]
    );
    return (
        <ConfirmationDialogProvider action={act}>
            <ReleaseLockButtonGroup lock={lock} queueHint={queueHint} />
        </ConfirmationDialogProvider>
    );
};

const ReleaseEnvironment = (props: {
    overview: GetOverviewResponse;
    applicationName: string;
    version: number; // the version we are currently looking at (not the version that is deployed)
    environmentName: string;
}) => {
    const { overview, applicationName, version, environmentName } = props;
    // deploy
    const act: BatchAction = useMemo(
        () => ({
            action: {
                $case: 'deploy',
                deploy: {
                    application: applicationName,
                    version: version,
                    environment: environmentName,
                    ignoreAllLocks: false,
                    lockBehavior: LockBehavior.Ignore,
                },
            },
        }),
        [applicationName, version, environmentName]
    );
    const currentlyDeployedVersion = overview.environments[environmentName].applications[applicationName]?.version;
    const queuedVersion = overview.environments[environmentName].applications[applicationName]?.queuedVersion;
    const hasQueue = queuedVersion !== 0;
    // delete queue is *almost* equal to deploy, just with a different version:
    const [queueEnv] = useUnaryCallback(
        React.useCallback(
            (api) =>
                api.deployService().Deploy({
                    application: applicationName,
                    version: currentlyDeployedVersion,
                    environment: environmentName,
                    lockBehavior: LockBehavior.Ignore,
                }),
            [applicationName, currentlyDeployedVersion, environmentName]
        )
    );
    const envLocks = Object.entries(overview.environments[environmentName].locks ?? {});
    const appLocks = Object.entries(overview.environments[environmentName]?.applications[applicationName]?.locks ?? {});
    const locked = envLocks.length > 0 || appLocks.length > 0;

    const button = (
        <ConfirmationDialogProvider action={act}>
            <DeployButton
                currentlyDeployedVersion={currentlyDeployedVersion}
                version={version}
                locked={locked}
                prefix={'deploy '}
                hasQueue={hasQueue}
            />
        </ConfirmationDialogProvider>
    );

    let currentInfo;
    if (currentlyDeployedVersion !== undefined) {
        const currentRelease = overview.applications[applicationName].releases.find(
            (r) => r.version === currentlyDeployedVersion
        );
        if (currentRelease !== undefined && currentRelease.sourceCommitId !== '') {
            currentInfo = (
                <>
                    <span className="commitId">{currentRelease.sourceCommitId}</span>
                    {': ' + currentRelease.sourceMessage}
                </>
            );
        }
    }

    appLocks.sort((a: [string, Lock], b: [string, Lock]) => {
        if (a[0] < b[0]) return -1;
        if (a[0] === b[0]) return 0;
        return 1;
    });
    envLocks.sort((a: [string, Lock], b: [string, Lock]) => {
        if (a[0] < b[0]) return -1;
        if (a[0] === b[0]) return 0;
        return 1;
    });

    const queueMessage =
        'Deletes the queue. ' +
        'Technically, it deploys the version that is already deployed here, which as a side effect deletes the queue.';
    return (
        <Paper className="environment">
            <Typography variant="h5" component="div" className="name" width="30%" sx={{ textTransform: 'capitalize' }}>
                {environmentName}
                <VersionDiff current={currentlyDeployedVersion} target={version} />
                <QueueDiff current={version} queued={queuedVersion} />
                {hasQueue ? (
                    <span>
                        <Tooltip title={queueMessage}>
                            <Button variant="contained" onClick={queueEnv} className={''}>
                                {'Delete Queue'}
                            </Button>
                        </Tooltip>
                    </span>
                ) : null}
            </Typography>
            <Typography variant="subtitle1" component="div" className="current">
                {currentInfo}
            </Typography>
            <ButtonGroup className="locks">
                {envLocks.map(([key, lock]) => (
                    <Tooltip arrow key={key} title={lock.message}>
                        <IconButton>
                            <LockIcon />
                        </IconButton>
                    </Tooltip>
                ))}
                {appLocks.map(([key, lock]) => (
                    <ReleaseLockButton
                        applicationName={applicationName}
                        environmentName={environmentName}
                        lock={lock}
                        lockId={key}
                        key={key}
                        queueHint={hasQueue}
                    />
                ))}
                <CreateLockButton applicationName={applicationName} environmentName={environmentName} />
            </ButtonGroup>
            <div className="buttons">{button}</div>
        </Paper>
    );
};

const useStyle = makeStyles((theme) => ({
    environments: {
        background: theme.palette.background.default,
        padding: theme.spacing(2),
        '& .environment': {
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
            padding: '1px 5px',
            '& .name': {
                width: '40%',
            },
            '& .current': {
                flexGrow: '1',
            },
            '& .locks': {
                '& .overlay': {
                    width: '400px',
                    '& .MuiTextField-root': {
                        width: '100%',
                    },
                },
            },
            '& .buttons': {},
            '& .same': {
                color: theme.palette.grey[500],
                fontSize: '1rem',
                margin: '0rem 0.5rem',
            },
            '& .notDeployed': {
                color: theme.palette.grey[500],
                fontSize: '1rem',
                margin: '0rem 0.5rem',
            },
            '& .ahead': {
                color: theme.palette.primary.dark,
                fontSize: '1rem',
                margin: '0rem 0.5rem',
            },
            '& .behind': {
                color: theme.palette.secondary.dark,
                fontSize: '1rem',
                margin: '0rem 0.5rem',
            },
        },
        '& .warning': {
            background: theme.palette.warning.main,
            color: theme.palette.warning.contrastText,
        },
        '& .warning:hover': {
            background: theme.palette.warning.dark,
        },
        '& .commitId': {
            color: theme.palette.grey[500],
            fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Menlo,Consolas,Liberation Mono,monospace',
        },
    },
    title: {
        '& .commitTimestamp': {
            color: theme.palette.grey[500],
            fontWeight: 'bold',
            borderRadius: '0.2rem',
            padding: '0rem 0.5rem',
            margin: '0.2rem 0.5rem',
            background: theme.palette.grey[900],
        },
        '& .commitId': {
            color: theme.palette.grey[500],
            fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Menlo,Consolas,Liberation Mono,monospace',
            fontWeight: 'bold',
            borderRadius: '0.2rem',
            padding: '0rem 0.5rem',
            margin: '0.2rem 0.5rem',
            background: theme.palette.grey[900],
        },
        '& .arrowNext': {
            float: 'left',
            margin: '0.5em 0 0 -1em',
        },
        '& .arrowPrev': {
            float: 'right',
            margin: '0.5em -1em 0 0',
        },
    },
}));

const ReleaseDialog = (props: {
    overview: GetOverviewResponse;
    applicationName: string;
    version: number;
    sortOrder: EnvSortOrder;
}) => {
    const { overview, applicationName, version, sortOrder } = props;
    const ctx = React.useContext(Context);
    const closeDialog = React.useCallback(() => {
        ctx.setData(null);
    }, [ctx]);
    const nextDialog = React.useCallback(() => {
        ctx.setData({ applicationName, version: version + 1 });
    }, [ctx, applicationName, version]);
    const prevDialog = React.useCallback(() => {
        ctx.setData({ applicationName, version: version - 1 });
    }, [ctx, applicationName, version]);

    const classes = useStyle();
    const envs = Object.values(overview.environments);
    const application: Application = overview.applications[applicationName];
    const release = application.releases.find((r) => r.version === version);
    const hasNextRelease = application.releases.find((r) => r.version > version) !== undefined;
    const hasPrevRelease = application.releases.find((r) => r.version < version) !== undefined;
    const sortedEnvs = sortEnvironmentsByUpstream(envs, sortOrder);
    const authorTime = release?.commit?.authorTime;
    const commitTime = authorTime
        ? authorTime?.getFullYear().toString() +
          '-' +
          (authorTime?.getMonth() + 1).toString().padStart(2, '0') +
          '-' +
          authorTime?.getDate().toString().padStart(2, '0') +
          ' ' +
          authorTime?.getHours().toString().padStart(2, '0') +
          ':' +
          authorTime?.getMinutes().toString().padStart(2, '0')
        : '';
    const timestamp = authorTime ? (
        <Tooltip arrow placement="right" title="Release Date">
            <div className="commitTimestamp">{commitTime}</div>
        </Tooltip>
    ) : (
        ''
    );

    return (
        <Dialog open fullWidth={true} maxWidth="lg">
            <DialogTitle className={classes.title}>
                <IconButton onClick={nextDialog} className="arrowNext" disabled={!hasNextRelease}>
                    <ArrowLeftIcon />
                </IconButton>
                <Typography variant="h2" component="span">
                    {applicationName}
                </Typography>
                <Typography variant="h2" className="version" component="span" sx={{ color: 'primary.main' }}>
                    {' ' + version}
                </Typography>
                <div style={{ display: 'inline-block' }}>
                    {timestamp}
                    <div className="commitId">{release?.sourceCommitId}</div>
                </div>
                <IconButton onClick={prevDialog} className="arrowPrev" disabled={!hasPrevRelease}>
                    <ArrowRightIcon />
                </IconButton>
            </DialogTitle>
            <DialogContent>
                <DialogContentText>{release?.sourceMessage}</DialogContentText>
            </DialogContent>
            <Grid container spacing={1} className={classes.environments}>
                {sortedEnvs.map((env) => (
                    <Grid item xs={12} key={env.name}>
                        <ReleaseEnvironment
                            applicationName={applicationName}
                            environmentName={env.name}
                            version={version}
                            overview={overview}
                        />
                    </Grid>
                ))}
            </Grid>
            <DialogActions>
                <Button onClick={closeDialog}>Close</Button>
            </DialogActions>
        </Dialog>
    );
};

export const ReleaseDialogProvider = (props: {
    overview: GetOverviewResponse;
    children: React.ReactNode;
    sortOrder: EnvSortOrder;
}) => {
    const [data, setData] = React.useState<{ applicationName: string; version: number } | null>(null);
    const dialog =
        data !== null ? (
            <ReleaseDialog
                key={data.applicationName + '-' + data.version}
                overview={props.overview}
                {...data}
                sortOrder={props.sortOrder}
            />
        ) : null;
    return (
        <Context.Provider value={{ setData }}>
            {props.children}
            {dialog}
        </Context.Provider>
    );
};

export const useOpen = (applicationName: string, version: number) => {
    const ctx = React.useContext(Context);
    return React.useCallback(() => {
        ctx.setData({ applicationName, version });
    }, [ctx, applicationName, version]);
};

export default ReleaseDialog;
