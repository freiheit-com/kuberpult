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
import { Releases } from './Releases';
import { render } from '@testing-library/react';
import { UpdateOverview } from '../../utils/store';
import { Release } from '../../../api/api';

describe('Release Dialog', () => {
    const data = [
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
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-05T12:30:12'),
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                },
            ] as Array<Release>,
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
                },
                {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-04T15:30:12'),
                },
                {
                    version: 3,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date('2022-12-06T12:30:12'),
                },
            ] as Array<Release>,
        },
    ];

    describe.each(data)(`Renders releases for an app`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { ['test']: { releases: testcase.releases } },
                environments: {},
            } as any);
            render(<Releases app="test" />);

            expect(document.querySelectorAll('.release_date')).toHaveLength(testcase.dates);
            expect(document.querySelectorAll('.content')).toHaveLength(testcase.releases.length);
        });
    });
});
