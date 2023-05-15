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
import { Spy } from 'spy4js';
import { elementQuerySelectorSafe, makeRelease } from '../../../setupTests';

const mock_FormattedDate = Spy.mockModule('../FormattedDate/FormattedDate', 'FormattedDate');

describe('Release Card Mini', () => {
    const getNode = (overrides: ReleaseCardMiniProps) => (
        <MemoryRouter>
            <ReleaseCardMini {...overrides} />
        </MemoryRouter>
    );
    const getWrapper = (overrides: ReleaseCardMiniProps) => render(getNode(overrides));

    type TestData = {
        name: string;
        expectedMessage: string;
        props: {
            app: string;
            version: number;
        };
        rels: Release[];
        environments: Environment[];
    };
    const data: TestData[] = [
        {
            name: 'using A release',
            props: { app: 'test2', version: 2 },
            rels: [makeRelease(2)],
            expectedMessage: 'test2',
            environments: [],
        },
        {
            name: 'A release three days ago with an env',
            props: { app: 'test2', version: 2 },
            rels: [makeRelease(2)],
            environments: [
                {
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
            ],
            expectedMessage: 'test2',
        },
        {
            name: 'A release with undeploy version',
            props: { app: 'test2', version: 2 },
            rels: [makeRelease(2, true)],
            environments: [
                {
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
            ],
            expectedMessage: 'Undeploy Version',
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
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
                environmentGroups: [
                    {
                        environments: testcase.environments,
                        distanceToUpstream: 2,
                        environmentGroupName: 'test-group',
                    },
                ],
            });
            const { container } = getWrapper(testcase.props);
            expect(container.querySelector('.release__details-mini')?.textContent).toContain(
                testcase.rels[0].sourceAuthor
            );
            expect(elementQuerySelectorSafe(container, '.env-group-chip-list-test').children.length).toBe(
                testcase.environments.length
            );
            expect(container.querySelector('.release__details-header')?.textContent).toBe(testcase.expectedMessage);
        });
    });
});
