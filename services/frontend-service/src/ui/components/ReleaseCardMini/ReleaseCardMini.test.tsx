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

Copyright freiheit.com*/
import { ReleaseCardMini, ReleaseCardMiniProps } from './ReleaseCardMini';
import { render } from '@testing-library/react';
import { AppDetailsState, updateAppDetails, UpdateOverview } from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';
import { Environment, Priority, Release, UndeploySummary } from '../../../api/api';
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
        expectedLabel: string | undefined;
        props: {
            app: string;
            version: number;
            revision: number;
        };
        rels: Release[];
        environments: Environment[];
    };
    const data: TestData[] = [
        {
            name: 'using A release',
            props: { app: 'test2', version: 2, revision: 0 },
            rels: [makeRelease(2, 'd1.2.3')],
            expectedMessage: 'test2',
            expectedLabel: 'd1.2.3 ',
            environments: [],
        },
        {
            name: 'with commit id',
            props: { app: 'test2', version: 2, revision: 0 },
            rels: [makeRelease(2, '')],
            expectedMessage: 'test2',
            expectedLabel: 'commit2 ',
            environments: [],
        },
        {
            name: 'without commit id, without displayVersion',
            props: { app: 'test2', version: 2, revision: 0 },
            rels: [makeRelease(2, '', '')],
            expectedMessage: 'test2',
            expectedLabel: '#2.0 ',
            environments: [],
        },
        {
            name: 'without commit id, without displayVersion, with some revision',
            props: { app: 'test2', version: 2, revision: 2 },
            rels: [makeRelease(2, '', '', false, 2)],
            expectedMessage: 'test2',
            expectedLabel: '#2.2 ',
            environments: [],
        },
        {
            name: 'A release three days ago with an env',
            props: { app: 'test2', version: 2, revision: 0 },
            rels: [makeRelease(2, '')],
            environments: [
                {
                    name: 'other',
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            expectedMessage: 'test2',
            expectedLabel: 'commit2 ',
        },
        {
            name: 'A release with undeploy version',
            props: { app: 'test2', version: 2, revision: 0 },
            rels: [makeRelease(2, '', '', true)],
            environments: [
                {
                    name: 'other',
                    distanceToUpstream: 0,
                    priority: 0,
                },
            ],
            expectedMessage: 'Undeploy Version',
            expectedLabel: 'undeploy ',
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            mock_FormattedDate.FormattedDate.returns(<div>some formatted date</div>);
            // when
            UpdateOverview.set({
                lightweightApps: [
                    {
                        name: 'test2',
                        team: 'example',
                    },
                ],
                environmentGroups: [
                    {
                        environments: testcase.environments,
                        distanceToUpstream: 2,
                        environmentGroupName: 'test-group',
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });

            updateAppDetails.set({
                test2: {
                    details: {
                        application: {
                            name: 'test2',
                            releases: testcase.rels,
                            sourceRepoUrl: 'http://test2.com',
                            team: 'example',
                            undeploySummary: UndeploySummary.NORMAL,
                            warnings: [],
                        },
                        deployments: {
                            other: {
                                version: 2,
                                revision: 0,
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        appLocks: {},
                        teamLocks: {},
                    },
                    appDetailState: AppDetailsState.READY,
                    updatedAt: new Date(Date.now()),
                    errorMessage: '',
                },
            });
            const { container } = getWrapper(testcase.props);
            expect(container.querySelector('.release__details-mini')?.textContent).toContain(
                testcase.rels[0].sourceAuthor
            );
            expect(elementQuerySelectorSafe(container, '.env-group-chip-list-test').children.length).toBe(
                testcase.environments.length
            );
            expect(container.querySelector('.release__details-header-title')?.textContent).toBe(
                testcase.expectedMessage
            );
            expect(container.querySelector('.links-left')?.textContent).toBe(testcase.expectedLabel);
        });
    });
});
