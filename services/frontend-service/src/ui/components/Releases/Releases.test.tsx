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
import { Releases } from './Releases';
import { render } from '@testing-library/react';
import { UpdateOverview } from '../../utils/store';
import { Release, UndeploySummary } from '../../../api/api';
import { MemoryRouter } from 'react-router-dom';

describe('Release Dialog', () => {
    type TestData = {
        name: string;
        dates: number;
        releases: Release[];
    };

    const data: TestData[] = [
        {
            name: '3 releases in 3 days',
            dates: 3,
            releases: [
                {
                    version: 1,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '1',
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-05T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '2',
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '3',
                },
            ],
        },
        {
            name: '3 releases in 2 days',
            dates: 2,
            releases: [
                {
                    version: 1,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '1',
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T15:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '2',
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                    undeployVersion: false,
                    prNumber: '666',
                    displayVersion: '3',
                },
            ],
        },
    ];

    describe.each(data)(`Renders releases for an app`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: {
                    test: {
                        releases: testcase.releases,
                        name: 'test',
                        sourceRepoUrl: 'url',
                        undeploySummary: UndeploySummary.NORMAL,
                        team: 'no-team',
                        warnings: [],
                    },
                },
            });
            render(
                <MemoryRouter>
                    <Releases app="test" />
                </MemoryRouter>
            );

            expect(document.querySelectorAll('.release_date')).toHaveLength(testcase.dates);
            expect(document.querySelectorAll('.content')).toHaveLength(testcase.releases.length);
        });
    });
});
