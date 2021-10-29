import React from 'react';
import { render } from '@testing-library/react';
import { Releases, calculateDistanceToUpstream, sortEnvironmentsByUpstream } from './Releases';
import { EnvSortOrder } from './Releases';
import {
    Environment, Environment_Application, Environment_Config_Upstream,
    GetOverviewResponse,
    Release,
} from '../api/api';

describe('Releases', () => {
    const getNode = () => {
        // given
        const dummyRelease1: Release = {
            version: 1,
            sourceCommitId: '12345687',
            sourceAuthor: 'testing test',
            sourceMessage: 'this is a test',
        }
        const dummyApp1: Environment_Application = {
            name: 'app1',
            version: 1,
            queuedVersion: 0,
            locks: {},
        }
        const dummyEnv: Environment = {
            name: 'env1',
            locks: {},
            applications: {
                'app1': dummyApp1
            },
        }
        const dummyOverview: GetOverviewResponse = {
            environments: {
                'env1': dummyEnv,
            },
            applications: {
                'app1':  {
                    name: 'app1',
                    releases: [dummyRelease1],
                }
            }
        }

        return <Releases data={dummyOverview} />;
    };
    const getWrapper = () => render(getNode());

    it('renders the releases component', () => {
        // when
        const { container } = getWrapper();

        // then
        expect(container.querySelector('.details')).toBeTruthy();
    });

    // testing the sort function for environments
    const getUpstream = (env: string): Environment_Config_Upstream => {
        return env === 'latest' ?
            {
                upstream: {
                    $case: 'latest',
                    latest: true,
                }
            }
            :
            {
                upstream: {
                    $case: 'environment',
                    environment: env,
                }
            }
    }

    const getEnvironment = (name: string, upstreamEnv?: string): Environment => {
        return {
            name: name,
            locks: {},
            applications: {},
            ...(upstreamEnv && {
                config: {
                    upstream: getUpstream(upstreamEnv)
                }
            }),
        }
    }

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
    }

    // Expected order / distance to upstream
    const chainOrder: EnvSortOrder = {
        'env4': 4,
        'env2': 2,
        'env0': 0,
        'env1': 1,
        'env3': 3,
    }
    const treeOrder: EnvSortOrder = {
        'env0': 2,
        'env1': 2,
        'env2': 1,
        'env3': 0,
        'env4': 0,
    }
    const cycleOrder: EnvSortOrder = {
        'env0': 6,
        'env1': 6,
        'env2': 1,
        'env3': 6,
        'env4': 0,
    }
    const noConfigOrder: EnvSortOrder = {
        'env0': 0,
        'env1': 0,
        'env2': 0,
        'env3': 0,
        'env4': 0,
    }

    const data = [
        {
            type: 'simple chain',
            envs: getEnvs('chain'),
            order: chainOrder,
            expect: ['env0', 'env1', 'env2', 'env3', 'env4'],
        },
        {
            type: 'tree',
            envs: getEnvs('tree'),
            order: treeOrder,
            expect: ['env3', 'env4', 'env2', 'env0', 'env1'],
        },
        {
            type: 'cycle',
            envs: getEnvs('cycle'),
            order: cycleOrder,
            expect: ['env4', 'env2', 'env0', 'env1', 'env3'],
        },
        {
            type: 'no-config',
            envs: getEnvs('no-config'),
            order: noConfigOrder,
            expect: ['env0', 'env1', 'env2', 'env3', 'env4'],
        },
    ];

    describe.each(data)(`Environment set`, (testcase) => {
        it(`with expected ${testcase.type} order`, () => {
            const sortedEnvs = sortEnvironmentsByUpstream(testcase.envs, testcase.order);
            const sortOrder = calculateDistanceToUpstream(testcase.envs);
            const sortedList = sortedEnvs.map(a => a.name);
            expect(sortOrder).toStrictEqual(testcase.order);
            expect(sortedList).toStrictEqual(testcase.expect);
        });
    });
});
