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
import { ReleaseCardMini, ReleaseCardMiniProps } from './ReleaseCardMini';
import { render } from '@testing-library/react';
import { UpdateOverview } from '../../utils/store';

describe('Release Card', () => {
    const getNode = (overrides: ReleaseCardMiniProps) => <ReleaseCardMini {...overrides} />;
    const getWrapper = (overrides: ReleaseCardMiniProps) => render(getNode(overrides));

    const data = [
        {
            name: 'A release from 2 days ago',
            props: { app: 'test1', version: 2 },
            msg: 'test-author commited 2 days ago at 14:20:0',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-14T14:20:00'),
                },
            ],
        },
        {
            name: 'A release from 4 days ago',
            props: { app: 'test1', version: 2 },
            msg: 'test-author commited 4 days ago at 8:20:0',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-12T08:20:00'),
                },
            ],
        },
        {
            name: 'using A release today',
            props: { app: 'test2', version: 2 },
            msg: 'test-author commited at 14:20:0',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-16T14:20:00'),
                },
            ],
        },
        {
            name: 'A release three days ago with an env',
            props: { app: 'test2', version: 2 },
            msg: 'test-author commited 3 days ago at 14:20:0',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-13T14:20:00'),
                },
            ],
            environments: {
                other: {
                    name: 'other',
                    applications: {
                        test2: {
                            version: 2,
                        },
                    },
                },
            },
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.environments ?? {},
            } as any);
            // Mock Date.now to always return 2022-12-16T14:20:00
            Date.now = jest.fn(() => Date.parse('2022-12-16T14:20:00'));
            const { container } = getWrapper(testcase.props);
            expect(container.querySelector('.release__details-mini')?.textContent).toContain(testcase.msg);
            if (testcase.environments?.other) {
                expect(container.querySelector('.release-environment')?.textContent).toContain('other');
            }
        });
    });
});
