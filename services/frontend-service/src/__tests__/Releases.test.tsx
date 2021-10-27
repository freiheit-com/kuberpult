import React from 'react';
import { render } from '@testing-library/react';
import Releases, {calculateDistanceToUpstream, sortEnvironmentsByUpstream} from '../ui/Releases';
import { EnvSortOrder } from '../ui/Releases';
import {
    Environment, Environment_Application, Environment_Config_Upstream,
    GetOverviewResponse,
    Release
} from "../api/api";

describe('Release Dialog', () => {
    const getNode = () => {
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
        const defaultProps: any  = {
            data: dummyOverview
        };

        return <Releases {...defaultProps} />;
    };
    const getWrapper = () => render(getNode());

    it('renders the release dialog', () => {
        // when
        const { container } = getWrapper();

        // then
        expect(container.querySelector('.details')).toBeTruthy();
    });

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

    const commonEnv = (n: number, cfg: string): Environment => {
        return {
                name: 'env' + n.toString(),
                locks: {},
                applications: {},
                config: {
                    upstream: getUpstream(cfg),
                },
            }
    }

    // original order 4, 2, 0, 1, 3
    const getEnvs = (testcase: string): Environment[] => {
        switch (testcase) {
            case 'chain':
                return [
                    commonEnv(4, 'env3'),
                    commonEnv(2, 'env1'),
                    commonEnv(0, 'latest'),
                    commonEnv(1, 'env0'),
                    commonEnv(3, 'env2'),
                ];
            case 'tree':
                return [
                    commonEnv(4, 'latest'),
                    commonEnv(2, 'env3'),
                    commonEnv(0, 'env2'),
                    commonEnv(1, 'env2'),
                    commonEnv(3, 'latest'),
                ];
            case 'cycle':
                return [
                    commonEnv(4, 'latest'),
                    commonEnv(2, 'env4'),
                    commonEnv(0, 'env3'),
                    commonEnv(1, 'env0'),
                    commonEnv(3, 'env1'),
                ];
            default:
                return [];
        }
    }

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
