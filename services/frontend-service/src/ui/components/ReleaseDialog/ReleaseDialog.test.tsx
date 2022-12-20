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
import {
    calculateDistanceToUpstream, calculateEnvironmentPriorities, EnvPrio,
    ReleaseDialog,
    ReleaseDialogProps,
    sortEnvironmentsByUpstream,
} from './ReleaseDialog';
import { render } from '@testing-library/react';
import { UpdateOverview, updateReleaseDialog } from '../../utils/store';
import { Environment, Environment_Config_Upstream, Release } from '../../../api/api';

describe('Release Dialog', () => {
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        rels: Release[];
        expect_message: boolean;
        data_length: number;
    }
    const data: dataT[] = [
        {
            name: 'normal release',
            props: {
                app: 'test1',
                version: 2,
                release: {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                },
                envs: [
                    {
                        name: 'prod',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 2,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                    },
                ],
            },
            rels: [],

            expect_message: true,
            data_length: 1,
        },
        {
            name: 'two envs release',
            props: {
                app: 'test1',
                version: 2,
                release: {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                },
                envs: [
                    {
                        name: 'prod',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 2,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                    },
                    {
                        name: 'dev',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 3,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                    },
                ],
            },
            rels: [],

            expect_message: true,
            data_length: 2,
        },
        {
            name: 'no release',
            props: {
                app: 'test1',
                version: -1,
                release: {} as Release,
                envs: [],
            },
            rels: [],
            expect_message: false,
            data_length: 0,
        },
    ];

    describe.each(data)(`Renders a Release Dialog`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            if (testcase.expect_message) {
                expect(document.querySelector('.release-dialog-message')?.textContent).toContain(
                    testcase.props.release.sourceMessage
                );
            } else {
                expect(document.querySelector('.release-dialog-message') === undefined);
            }
            expect(document.querySelectorAll('.env-card-data')).toHaveLength(testcase.data_length);
        });
    });

    describe.each(data)(`Renders the environment cards`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            expect(document.querySelector('.release-env-list')?.children).toHaveLength(testcase.props.envs.length);
        });
    });

    describe.each(data)(`Renders the environment locks`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            testcase.props.envs.forEach((env) => {
                expect(document.querySelector('.env-card-env-locks')?.children).toHaveLength(
                    Object.values(env.locks).length
                );
            });
        });
    });
});

// testing the sort function for environments
const getUpstream = (env: string): Environment_Config_Upstream =>
    env === 'latest'
        ? {
              upstream: {
                  $case: 'latest',
                  latest: true,
              },
          }
        : {
              upstream: {
                  $case: 'environment',
                  environment: env,
              },
          };

const getEnvironment = (name: string, upstreamEnv?: string): Environment => ({
    name: name,
    locks: {},
    applications: {},
    ...(upstreamEnv && {
        config: {
            upstream: getUpstream(upstreamEnv),
        },
    }),
});

// original order [ 4, 2, 0, 1, 3 ]
const getEnvs = (testcase: string): Environment[] => {
    switch (testcase) {
        case 'chain':
            return [
                getEnvironment('env4', 'env3'),
                getEnvironment('env2', 'env1'),
                getEnvironment('env0', 'latest'),
                getEnvironment('env1', 'env0'),
                getEnvironment('env3', 'env2'),
            ];
        case 'tree':
            return [
                getEnvironment('env4', 'latest'),
                getEnvironment('env2', 'env3'),
                getEnvironment('env0', 'env2'),
                getEnvironment('env1', 'env2'),
                getEnvironment('env3', 'latest'),
            ];
        case 'cycle':
            return [
                getEnvironment('env4', 'latest'),
                getEnvironment('env2', 'env4'),
                getEnvironment('env0', 'env3'),
                getEnvironment('env1', 'env0'),
                getEnvironment('env3', 'env1'),
            ];
        case 'no-config':
            return [
                getEnvironment('env4'),
                getEnvironment('env2'),
                getEnvironment('env0'),
                getEnvironment('env1'),
                getEnvironment('env3'),
            ];
        default:
            throw new Error("bad test: " + testcase);
    }
};

const sortByUpstreamData = [
    {
        type: 'simple chain',
        envs: getEnvs('chain'),
        expect: ['env0', 'env1', 'env2', 'env3', 'env4'],
    },
    {
        type: 'simple tree',
        envs: getEnvs('tree'),
        expect: ['env3', 'env4', 'env2', 'env0', 'env1'],
    },
    {
        type: 'simple cycle',
        envs: getEnvs('cycle'),
        expect: ['env4', 'env2', 'env0', 'env1', 'env3'],
    },
    {
        type: 'no-config',
        envs: getEnvs('no-config'),
        expect: ['env0', 'env1', 'env2', 'env3', 'env4'],
    },
];

describe.each(sortByUpstreamData)(`Environment set`, (testcase) => {
    it(`with expected ${testcase.type} order`, () => {
        const sortedEnvs = sortEnvironmentsByUpstream(testcase.envs);
        calculateDistanceToUpstream(testcase.envs);
        const sortedList = sortedEnvs.map((a) => a.name);
        expect(sortedList).toStrictEqual(testcase.expect);
    });
});


const calcEnvPrioData = [
    {
        type: 'chain',
        expect: {'env0': EnvPrio.UPSTREAM, 'env4': EnvPrio.PROD, 'env3': EnvPrio.PRE_PROD, 'env2': EnvPrio.OTHER, 'env1': EnvPrio.OTHER },
    },
    {
        type: 'tree',
        expect: {'env0': EnvPrio.PROD, 'env4': EnvPrio.UPSTREAM, 'env3': EnvPrio.UPSTREAM, 'env2': EnvPrio.PRE_PROD, 'env1': EnvPrio.PROD },
    },
    {
        // the main point here is that it doesn't crash
        // we do not fully support disconnected trees
        type: 'cycle',
        expect: {'env0': EnvPrio.PROD, 'env4': EnvPrio.UPSTREAM, 'env3': EnvPrio.PROD, 'env2': EnvPrio.OTHER, 'env1': EnvPrio.PROD },
    },
    {
        // the main point here is that it doesn't crash
        type: 'no-config',
        expect: {'env0': EnvPrio.PROD, 'env4': EnvPrio.PROD, 'env3': EnvPrio.PROD, 'env2': EnvPrio.PROD, 'env1': EnvPrio.PROD },
    },
];

describe.each(calcEnvPrioData)(`test calculateEnvironmentPriorities`, (testcase) => {
    it(`with expected ${testcase.type}`, () => {
        const envs = getEnvs(testcase.type);
        const actualPriorities = calculateEnvironmentPriorities(envs);
        expect(actualPriorities).toStrictEqual(testcase.expect);
    });
});
