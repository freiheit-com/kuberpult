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
import { Application, Release } from '../../../api/api';
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
                version: -1,
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
} as any;

describe('Service Lane', () => {
    const getNode = (overrides: { application: Application }) => (
        <MemoryRouter>
            <ServiceLane {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: { application: Application }) => render(getNode(overrides));
    it('Renders a row of releases', () => {
        // when
        UpdateOverview.set({
            environments: sampleEnvs,
        });
        const sampleApp = {
            name: 'test2',
            releases: [],
            sourceRepoUrl: 'http://test2.com',
            team: 'example',
        };
        getWrapper({ application: sampleApp });

        // then releases are sorted and Release card is called with props:
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(0, 0)).toStrictEqual({ app: sampleApp.name, version: -1 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(1, 0)).toStrictEqual({ app: sampleApp.name, version: 3 });
        expect(mock_ReleaseCard.ReleaseCard.getCallArgument(2, 0)).toStrictEqual({ app: sampleApp.name, version: 2 });
        mock_ReleaseCard.ReleaseCard.wasCalled(3);
    });
});

const data = [
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
            },
        ] as Array<Release>,
        envs: {
            foo: {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                    },
                },
            },
            foo2: {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 1,
                    },
                },
            },
        } as any,
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
            },
            {
                version: 2,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
            },
        ] as Array<Release>,
        envs: {
            foo: {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                    },
                },
            },
            foo2: {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 2,
                    },
                },
            },
        } as any,
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
            },
            {
                version: 4,
                sourceMessage: 'test5',
                sourceAuthor: 'test',
                sourceCommitId: 'commit5',
                createdAt: new Date(2002),
            },
            {
                version: 2,
                sourceMessage: 'test3',
                sourceAuthor: 'test',
                sourceCommitId: 'commit3',
                createdAt: new Date(2002),
            },
        ] as Array<Release>,
        envs: {
            foo: {
                name: 'foo',
                applications: {
                    test2: {
                        version: 1,
                    },
                },
            },
            foo2: {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 4,
                    },
                },
            } as any,
        },
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
            },
            {
                version: 4,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
            },
            {
                version: 3,
                sourceMessage: 'test2',
                sourceAuthor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
            },
            {
                version: 5,
                sourceMessage: 'test2',
                sourceAuhor: 'test',
                sourceCommitId: 'commit2',
                createdAt: new Date(2002),
            },
        ] as Array<Release>,
        envs: {
            foo: {
                name: 'foo',
                applications: {
                    test2: {
                        version: 2,
                    },
                },
            },
            foo2: {
                name: 'foo2',
                applications: {
                    test2: {
                        version: 5,
                    },
                },
            } as any,
        },
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
                environments: testcase.envs,
                applications: { test2: { releases: testcase.releases } },
            });
            const sampleApp = {
                name: 'test2',
                releases: [],
                sourceRepoUrl: 'http://test2.com',
                team: 'example',
            };
            const { container } = getWrapper({ application: sampleApp });

            // check for the diff between versions
            if (testcase.diff === '-1') {
                expect(document.querySelector('.service-lane__diff_number') === undefined);
            } else {
                expect(container.querySelector('.service-lane__diff_number')?.textContent).toContain(testcase.diff);
            }
        });
    });
});
