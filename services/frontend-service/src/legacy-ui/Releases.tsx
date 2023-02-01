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

import Avatar from '@material-ui/core/Avatar';
import AvatarGroup from '@material-ui/core/AvatarGroup';
import Badge from '@material-ui/core/Badge';
import Paper from '@material-ui/core/Paper';
import LockIcon from '@material-ui/icons/Lock';
import Table from '@material-ui/core/Table';
import TableBody from '@material-ui/core/TableBody';
import TableCell from '@material-ui/core/TableCell';
import TableContainer from '@material-ui/core/TableContainer';
import TableRow from '@material-ui/core/TableRow';
import Tooltip from '@material-ui/core/Tooltip';

import { ReleaseDialogProvider, useOpen } from './ReleaseDialog';

import type { Application, Environment, Release, GetOverviewResponse } from '../api/api';
import { Warnings } from './Warnings';
import { useMemo } from 'react';
import { ConfirmationDialogProvider } from './ConfirmationDialog';
import Button from '@material-ui/core/Button';
import { CartAction } from './ActionDetails';
import { useSearchParams } from 'react-router-dom';
export type EnvSortOrder = { [index: string]: number };

const useStyles = makeStyles((theme) => ({
    root: {
        '& .AppBarSpacer': theme.mixins.toolbar,
        '& .application': {
            margin: theme.spacing(0.5),
            minHeight: '100px',
        },
        '& .applicationCard': {
            padding: theme.spacing(1, 2),
            margin: theme.spacing(0, 2),
            borderRight: '1px solid ' + theme.palette.divider,
            minWidth: '200px',
            position: 'sticky',
            left: 0,
            zIndex: 2,
        },
        '& .releases': {
            display: 'flex',
            flexDirection: 'row',
        },
        '& .release': {
            width: '100px',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'flex-start',
            marginRight: theme.spacing(1),
            cursor: 'pointer',
            zIndex: 1,
        },
        '& .release:hover': {
            boxShadow: theme.shadows[20],
            marginTop: theme.spacing(-0.5),
            marginBottom: theme.spacing(0.5),
            zIndex: 2,
        },
        '& .release .version': {
            border: '5px solid ' + theme.palette.divider,
            marginTop: '-20px',
            marginBottom: '10px',
        },
        '& .release .details': {
            width: '100%',
            height: '40px',
            background: theme.palette.grey[700],
            borderBottom: '5px solid ' + theme.palette.divider,
            borderRadius: '' + theme.shape.borderRadius + 'px ' + theme.shape.borderRadius + 'px 0 0',
            display: 'flex',
            justifyContent: 'center',
            '& .commitId': {
                color: theme.palette.grey[400],
                fontFamily: 'ui-monospace,SFMono-Regular,SF Mono,Menlo,Consolas,Liberation Mono,monospace',
            },
        },
        '& .release .version-new': {
            background: '#61a300',
        },
        '& .release .version-medium': {
            background: '#2c7f44',
        },
        '& .release .version-old': {
            background: '#25332b',
        },
        '& .release .version-history': {
            background: theme.palette.grey[700],
        },
        '& .release .envs': {
            minHeight: '40px',
        },
        '& .release .envs .MuiAvatar-root': {
            width: '24px',
            height: '24px',
            fontSize: '0.95rem',
            fontWeight: 'bold',
            backgroundColor: theme.palette.secondary.light,
            borderColor: theme.palette.secondary.dark,
            color: theme.palette.secondary.contrastText,
            position: 'relative',
        },
        '& .release .envs .MuiBadge-anchorOriginBottomRightCircular': {
            bottom: '30%',
            right: '30%',
            '& svg': {
                fontSize: '12px',
            },
        },
        '& .applicationCard env': {
            height: theme.spacing(1),
            width: '100%',
        },
        '& .ownerText': {
            color: '#808080',
        },
    },
}));

const getFreshnessColor = (authorTime?: Date): string => {
    if (!authorTime) return 'version-history';
    const diff = Date.now().valueOf() - authorTime.valueOf();
    const HOUR = 60 * 60 * 1000;
    if (diff.valueOf() < 2 * HOUR) {
        return 'version-new';
    }
    if (diff.valueOf() < 24 * HOUR) {
        return 'version-medium';
    }
    if (diff.valueOf() < 24 * 7 * HOUR) {
        return 'version-old';
    }
    return 'version-history';
};

const ReleaseBox = (props: { name: string; release: Release; envs: Array<Environment>; sortOrder: EnvSortOrder }) => {
    const { name, release, envs, sortOrder } = props;
    const openReleaseBox = useOpen(name, release.version);
    const sortedEnvs = sortEnvironmentsByUpstream(envs, sortOrder);

    return (
        <Tooltip title={release.sourceMessage} arrow>
            <Paper key={release.version} className="release" onClick={openReleaseBox}>
                <div className={'details'}>
                    <span className="commitId">{release.sourceCommitId}</span>
                </div>
                <Avatar className={'version ' + getFreshnessColor(release.createdAt)} />
                <AvatarGroup className="envs">
                    {sortedEnvs.map((env) => (
                        <EnvAvatar env={env} application={props.name} key={env.name} />
                    ))}
                </AvatarGroup>
            </Paper>
        </Tooltip>
    );
};

const UndeployButton = (props: {
    addToCart?: () => void; //
    inCart?: boolean; //
    applicationName: string; //
}) => {
    const buttonMsg = 'Prepare to Undeploy';
    const tooltipMsg =
        'This will create a new version that is empty. Use this only for services that are not needed anymore.';
    return (
        <Tooltip title={tooltipMsg}>
            <Button variant="contained" onClick={props.addToCart} disabled={props.inCart}>
                <span style={{ fontSize: '0.5rem' }}>{buttonMsg}</span>
            </Button>
        </Tooltip>
    );
};

const ApplicationBox: React.FC<any> = (props: {
    name: string;
    environments: { [name: string]: Environment };
    application: Application;
    sortOrder: EnvSortOrder;
}) => {
    const { name, environments, application, sortOrder } = props;
    const envsPerVersion = new Map<Number, Array<string>>();
    for (const k in environments) {
        const a = environments[k].applications[name];
        if (a) {
            const envs = envsPerVersion.get(a.version);
            if (!envs) {
                envsPerVersion.set(a.version, [k]);
            } else {
                envs.push(k);
            }
        }
    }
    const releases = application.releases;
    releases?.sort((a, b) => b.version - a.version);
    const warnings = <Warnings name={name} environments={environments} releases={releases} />;
    const undeployAction: CartAction = useMemo(
        () => ({
            prepareUndeploy: {
                application: name,
            },
        }),
        [name]
    );
    const undeployButton = (
        <ConfirmationDialogProvider action={undeployAction}>
            <UndeployButton applicationName={name} />
        </ConfirmationDialogProvider>
    );

    const owner = <div className="ownerText">{application.team !== '' ? 'Owner: ' + application.team : null}</div>;

    return (
        <TableRow className="application">
            <TableCell className="applicationCard">
                {warnings}
                {name}
                {undeployButton}
                {owner}
            </TableCell>
            <TableCell className="releases">
                {releases?.map((release) => (
                    <ReleaseBox
                        name={name}
                        release={release}
                        key={release.version}
                        envs={envsPerVersion.get(release.version)?.map((env) => environments[env]) ?? []}
                        sortOrder={sortOrder}
                    />
                ))}
            </TableCell>
        </TableRow>
    );
};

const EnvAvatar = (props: { env: Environment; application: string }) => {
    const { env, application } = props;
    const locked = Object.keys(env.locks).length > 0 || Object.keys(env.applications[application]?.locks).length > 0;
    const ava = <Avatar variant="rounded">{env.name.substring(0, 1).toUpperCase()}</Avatar>;
    if (locked) {
        return (
            <Badge
                overlap="circular"
                anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                badgeContent={<LockIcon />}>
                {ava}
            </Badge>
        );
    } else {
        return ava;
    }
};

export const calculateDistanceToUpstream = (envs: Environment[]): EnvSortOrder => {
    const distanceToUpstream: EnvSortOrder = {};
    let rest: Environment[] = [];
    for (const env of envs) {
        if (!env.config?.upstream?.upstream?.$case || env.config?.upstream?.upstream?.$case === 'latest') {
            distanceToUpstream[env.name] = 0;
        } else {
            rest.push(env);
        }
    }
    // iterate over rest until nothing is left
    while (rest.length > 0) {
        const nextRest: Environment[] = [];
        for (const env of rest) {
            const upstreamEnv = (env.config?.upstream?.upstream as any).environment;
            if (upstreamEnv in distanceToUpstream) {
                distanceToUpstream[env.name] = distanceToUpstream[upstreamEnv] + 1;
            } else {
                nextRest.push(env);
            }
        }
        if (rest.length === nextRest.length) {
            // infinite loop here, maybe fill in the remaining entries with max(distanceToUpstream) + 1
            for (const env of rest) {
                distanceToUpstream[env.name] = envs.length + 1;
            }
            return distanceToUpstream;
        }
        rest = nextRest;
    }
    return distanceToUpstream;
};

/**
 * @deprecated only use in legacy ui!
 */
export const sortEnvironmentsByUpstream = (envs: Environment[], distance: EnvSortOrder): Environment[] => {
    const sortedEnvs = [...envs];
    sortedEnvs.sort((a: Environment, b: Environment) => {
        if (distance[a.name] === distance[b.name]) {
            if (a.name < b.name) return -1;
            if (a.name === b.name) return 0;
            return 1;
        }
        if (distance[a.name] < distance[b.name]) return -1;
        return 1;
    });
    return sortedEnvs;
};

export const Releases: React.FC<any> = (props: { data: GetOverviewResponse }) => {
    const { data } = props;
    const classes = useStyles(data.environments);
    const [searchParams] = useSearchParams();
    let apps = Object.values(data.applications);

    if (searchParams.has('app')) {
        apps = apps.filter((k) => k.name.includes(searchParams.get('app')!));
    }
    if (searchParams.has('team')) {
        apps = apps.filter((k) => k.team.includes(searchParams.get('team')!));
    }

    // Sort by team, within team sort by name
    apps.sort((a, b) => a.team?.localeCompare(b.team) || a.name?.localeCompare(b.name) || 0);
    // calculate the distances with all envs before sending only subsets of the envs into release boxes
    // only run once per refresh
    const sortOrder = calculateDistanceToUpstream(Object.values(data.environments));

    return (
        <ReleaseDialogProvider overview={data} sortOrder={sortOrder}>
            <TableContainer>
                <Table>
                    <TableBody className={classes.root}>
                        {apps.map((app) => (
                            <ApplicationBox
                                key={app.name}
                                name={app.name}
                                application={app}
                                environments={data.environments}
                                sortOrder={sortOrder}
                            />
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>
        </ReleaseDialogProvider>
    );
};
export default Releases;
