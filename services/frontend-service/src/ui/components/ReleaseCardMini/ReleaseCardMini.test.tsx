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
import { ReleaseCardMini, ReleaseCardMiniProps } from './ReleaseCardMini';
import { render } from '@testing-library/react';
import { UpdateOverview } from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';
import { Environment, Release, UndeploySummary } from '../../../api/api';

describe('Release Card Mini', () => {
    const getNode = (overrides: ReleaseCardMiniProps) => (
        <MemoryRouter>
            <ReleaseCardMini {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseCardMiniProps) => render(getNode(overrides));

    type TestData = {
        name: string;
        msg: string;
        expectedMessage: string;
        props: {
            app: string;
            version: number;
        };
        rels: Release[];
        environments: { [key: string]: Environment };
    };
    const data: TestData[] = [
        {
            name: 'A release from 2 days ago',
            props: { app: 'test1', version: 2 },
            msg: 'test-reltest-author | 2022-12-14 @ 14:20 | 2 days ago',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    prNumber: '666',
                    createdAt: new Date('2022-12-14T14:20:00'),
                },
            ],
            expectedMessage: 'test-rel',
            environments: {},
        },
        {
            name: 'A release from 4 days ago',
            props: { app: 'test1', version: 2 },
            msg: 'test-reltest-author | 2022-12-12 @ 8:20 | 4 days ago',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    prNumber: '666',
                    createdAt: new Date('2022-12-12T08:20:00'),
                },
            ],
            expectedMessage: 'test-rel',
            environments: {},
        },
        {
            name: 'using A release today',
            props: { app: 'test2', version: 2 },
            msg: 'test-reltest-author | 2022-12-16 @ 14:20 | < 1 hour ago',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-16T14:20:00'),
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    prNumber: '666',
                },
            ],
            expectedMessage: 'test-rel',
            environments: {},
        },
        {
            name: 'A release three days ago with an env',
            props: { app: 'test2', version: 2 },
            msg: 'test-reltest-author | 2022-12-13 @ 14:20 | 3 days ago',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-13T14:20:00'),
                    undeployVersion: false,
                    sourceCommitId: 'commit123',
                    prNumber: '666',
                },
            ],
            environments: {
                other: {
                    name: 'other',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        test2: {
                            version: 2,
                            queuedVersion: 0,
                            name: 'test2',
                            locks: {},
                            undeployVersion: false,
                        },
                    },
                },
            },
            expectedMessage: 'test-rel',
        },
        {
            name: 'A release with undeploy version',
            props: { app: 'test2', version: 2 },
            msg: 'test-author | 2022-12-13 @ 14:20 | 3 days ago',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-13T14:20:00'),
                    sourceCommitId: 'commit123',
                    prNumber: '666',
                    undeployVersion: true,
                },
            ],
            environments: {
                other: {
                    name: 'other',
                    locks: {},
                    distanceToUpstream: 0,
                    priority: 0,
                    applications: {
                        test2: {
                            version: 2,
                            queuedVersion: 0,
                            name: 'test2',
                            locks: {},
                            undeployVersion: false,
                        },
                    },
                },
            },
            expectedMessage: 'Undeploy Version',
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: {
                    [testcase.props.app]: {
                        name: testcase.props.app,
                        releases: testcase.rels,
                        sourceRepoUrl: 'url',
                        undeploySummary: UndeploySummary.Normal,
                        team: 'no-team',
                    },
                },
                environments: testcase.environments ?? {},
                environmentGroups: [],
            });
            // Mock Date.now to always return 2022-12-16T14:20:00
            Date.now = jest.fn(() => Date.parse('2022-12-16T14:20:00'));
            const { container } = getWrapper(testcase.props);
            expect(container.querySelector('.release__details-mini')?.textContent).toContain(testcase.msg);
            expect(container.querySelector('.env-group-chip-list-test')).not.toBeEmptyDOMElement();
            expect(container.querySelector('.release__details-header')?.textContent).toBe(testcase.expectedMessage);
        });
    });
});
