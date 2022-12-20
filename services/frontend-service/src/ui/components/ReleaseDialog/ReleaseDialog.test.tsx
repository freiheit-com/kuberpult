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
    calculateDistanceToUpstream,
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
            return [];
    }
};

const data = [
    {
        type: 'simple chain',
        envs: getEnvs('chain'),
        expect: ['env4', 'env3', 'env2', 'env1', 'env0'],
    },
    {
        type: 'simple tree',
        envs: getEnvs('tree'),
        expect: ['env1', 'env0', 'env2', 'env4', 'env3'],
    },
    {
        type: 'simple cycle',
        envs: getEnvs('cycle'),
        expect: ['env3', 'env1', 'env0', 'env2', 'env4'],
    },
    {
        type: 'no-config',
        envs: getEnvs('no-config'),
        expect: ['env4', 'env3', 'env2', 'env1', 'env0'],
    },
];

describe.each(data)(`Environment set`, (testcase) => {
    it(`with expected ${testcase.type} order`, () => {
        const sortedEnvs = sortEnvironmentsByUpstream(testcase.envs);
        calculateDistanceToUpstream(testcase.envs);
        const sortedList = sortedEnvs.map((a) => a.name);
        expect(sortedList).toStrictEqual(testcase.expect);
    });
});
