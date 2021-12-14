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
import React from 'react';
import { render } from '@testing-library/react';
import { Releases, calculateDistanceToUpstream, sortEnvironmentsByUpstream, EnvSortOrder } from './Releases';
import {
    Environment,
    Environment_Application,
    Environment_Config_Upstream,
    GetOverviewResponse,
    Release,
} from '../api/api';

describe('Releases', () => {
    const getRelease = (t?: Date) => {
        const r: Release = {
            version: 1,
            sourceCommitId: '12345687',
            sourceAuthor: 'testing test',
            sourceMessage: 'this is a test',
            undeployVersion: false,
        };
        if (t) {
            r.commit = {
                authorEmail: 'randomemail@example.com',
                authorName: 'random',
                authorTime: t,
            };
        }
        return r;
    };
    const dummyApp1: Environment_Application = {
        name: 'app1',
        version: 1,
        queuedVersion: 0,
        locks: {},
        undeployVersion: false,
    };
    const dummyEnv: Environment = {
        name: 'env1',
        locks: {},
        applications: {
            app1: dummyApp1,
        },
    };
    const getDummyOverview = (t?: Date) => {
        const r: GetOverviewResponse = {
            environments: {
                env1: dummyEnv,
            },
            applications: {
                app1: {
                    name: 'app1',
                    releases: [getRelease(t)],
                },
            },
        };
        return r;
    };

    const getNode = (overrides?: { data: GetOverviewResponse }) => {
        const defaultProps = { data: getDummyOverview() };
        return <Releases {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { data: GetOverviewResponse }) => render(getNode(overrides));

    it('renders the releases component', () => {
        // when
        const { container } = getWrapper();

        // then
        expect(container.querySelector('.details')).toBeTruthy();
    });

    // given
    const hr = 60 * 60 * 1000;
    const releasesWithDatesData = [
        {
            type: 'just created',
            overview: getDummyOverview(new Date()),
            expectedClassname: '.details-new',
        },
        {
            type: 'one hour old',
            overview: getDummyOverview(new Date(Date.now() - hr)),
            expectedClassname: '.details-new',
        },
        {
            type: '12 hours old',
            overview: getDummyOverview(new Date(Date.now() - 12 * hr)),
            expectedClassname: '.details-medium',
        },
        {
            type: '2 days old',
            overview: getDummyOverview(new Date(Date.now() - 48 * hr)),
            expectedClassname: '.details-old',
        },
        {
            type: '10 days old',
            overview: getDummyOverview(new Date(Date.now() - 10 * 24 * hr)),
            expectedClassname: '.details-history',
        },
        {
            type: 'un-specified date',
            overview: getDummyOverview(),
            expectedClassname: '.details-history',
        },
    ];

    describe.each(releasesWithDatesData)(`Releases with commit dates`, (testcase) => {
        it(`when ${testcase.type}`, () => {
            // when
            const { container } = getWrapper({ data: testcase.overview });

            // then
            expect(container.querySelector('.releases ' + testcase.expectedClassname)).toBeTruthy();
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

    // Expected order / distance to upstream
    const chainOrder: EnvSortOrder = {
        env4: 4,
        env2: 2,
        env0: 0,
        env1: 1,
        env3: 3,
    };
    const treeOrder: EnvSortOrder = {
        env0: 2,
        env1: 2,
        env2: 1,
        env3: 0,
        env4: 0,
    };
    const cycleOrder: EnvSortOrder = {
        env0: 6,
        env1: 6,
        env2: 1,
        env3: 6,
        env4: 0,
    };
    const noConfigOrder: EnvSortOrder = {
        env0: 0,
        env1: 0,
        env2: 0,
        env3: 0,
        env4: 0,
    };

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
            const sortedList = sortedEnvs.map((a) => a.name);
            expect(sortOrder).toStrictEqual(testcase.order);
            expect(sortedList).toStrictEqual(testcase.expect);
        });
    });
});
