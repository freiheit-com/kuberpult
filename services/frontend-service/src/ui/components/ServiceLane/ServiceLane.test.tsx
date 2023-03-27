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

Copyright 2023 freiheit.com*/
import { render } from '@testing-library/react';
import { ServiceLane } from './ServiceLane';
import { UpdateOverview } from '../../utils/store';
import { Spy } from 'spy4js';
import { Application, Environment, Priority, Release } from '../../../api/api';
import { MemoryRouter } from 'react-router-dom';

const mock_ReleaseCard = Spy.mockReactComponents('../../components/ReleaseCard/ReleaseCard', 'ReleaseCard');
const sampleEnvs = {
    foo: {
        // third release card contains two environments
        name: 'foo',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    foo2: {
        // third release card contains two environments
        name: 'foo2',
        applications: {
            test2: {
                version: 2,
            },
        },
    },
    bar: {
        // second release card contains one environment, newest version
        name: 'bar',
        applications: {
            test2: {
                version: 3,
            },
        },
    },
    undeploy: {
        // first release card is for the undeploy one
        name: 'undeploy',
        applications: {
            test2: {
                version: 5,
                undeployVersion: true,
            },
        },
    },
    other: {
        // no release card for different app
        name: 'other',
        applications: {
            test3: {
                version: 3,
            },
        },
    },
};

describe('Service Lane', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        const sampleApp = {
            name: 'test2',
            releases: [{ version: 5 }, { version: 2 }, { version: 3 }],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
        } as any;
        UpdateOverview.set({
            environments: sampleEnvs,
            applications: {
                test2: sampleApp,
            },
        });
        getWrapper({ application: sampleApp });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({ app: sampleApp.name, version: 5 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({ app: sampleApp.name, version: 3 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({ app: sampleApp.name, version: 2 });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });
});

type TestData = {
    name: string;
    diff: string;
    releases: Release[];
    envs: Environment[];
};

const data: TestData[] = [
    {
        name: 'test same version',
        diff: '-1',
        releases: [
            {
                version: 1,
                sourceMessage: 'test1',
                sourceAuthor: 'test',
                sourceCommitId: 'commit1',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
        ],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
    {
        name: 'test no diff',
        diff: '0',
        releases: [
            {
                version: 1,
                sourceMessage: 'test1',
                sourceAuthor: 'test',
                sourceCommitId: 'commit1',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 2,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
        ],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 2,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
    {
        name: 'test diff by one',
        diff: '1',
        releases: [
            {
                version: 1,
                sourceMessage: 'test1',
                sourceAuthor: 'test',
                sourceCommitId: 'commit1',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 4,
                sourceMessage: 'test5',
                sourceAuthor: 'test',
                sourceCommitId: 'commit5',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 2,
                sourceMessage: 'test3',
                sourceAuthor: 'test',
                sourceCommitId: 'commit3',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
        ],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                    },
                },
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 4,
                    },
                },
                locks: {},
            } as any,
        ],
    },
    {
        name: 'test diff by two',
        diff: '2',
        releases: [
            {
                version: 2,
                sourceMessage: 'test1',
                sourceAuthor: 'test',
                sourceCommitId: 'commit1',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 4,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 3,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
            {
                version: 5,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
                undeployVersion: false,
                prNumber: '666',
            },
        ],
        envs: [
            {
                name: 'foo',
                applications: {
                    test2: {
                        version: 2,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
            {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 5,
                        name: '',
                        locks: {},
                        queuedVersion: 0,
                        undeployVersion: false,
                    },
                },
                distanceToUpstream: 0,
                priority: Priority.UPSTREAM,
                locks: {},
            },
        ],
    },
];

describe('Service Lane Diff', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    describe.each(data)('Service Lane diff', (testcase) => {
        it(testcase.name, () => {
            UpdateOverview.set({
                environments: undefined, // deprecated
                applications: { test2: { releases: testcase.releases, name: '', team: '', sourceRepoUrl: '' } },
                environmentGroups: [
                    {
                        environments: testcase.envs,
                        environmentGroupName: 'group1',
                        distanceToUpstream: 0,
                    },
                ],
            });
            const sampleApp = {
                name: 'test2',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
            };
            const { container } = getWrapper({ application: sampleApp });

            // check for the diff between versions
            if (testcase.diff === '-1' || testcase.diff === '0') {
                expect(document.querySelector('.service-lane__diff--number') === undefined);
            } else {
                expect(container.querySelector('.service-lane__diff--number')?.textContent).toContain(testcase.diff);
            }
        });
    });
});
