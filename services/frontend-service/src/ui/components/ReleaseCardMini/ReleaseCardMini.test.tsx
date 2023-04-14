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

describe('Release Card Mini', () => {
    const getNode = (overrides: ReleaseCardMiniProps) => (
        <MemoryRouter>
            <ReleaseCardMini {...overrides} />
        </MemoryRouter>
    );
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
            expectedMessage: 'test-rel',
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
            expectedMessage: 'test-rel',
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
            expectedMessage: 'test-rel',
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
            expectedMessage: 'test-rel',
        },
        {
            name: 'A release with undeploy version',
            props: { app: 'test2', version: 2 },
            msg: 'test-author commited 3 days ago at 14:20:0',
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceAuthor: 'test-author',
                    createdAt: new Date('2022-12-13T14:20:00'),
                    undeployVersion: true,
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
            expectedMessage: 'Undeploy Version',
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            // eslint-disable-next-line no-type-assertion/no-type-assertion
            UpdateOverview.set({
                // eslint-disable-next-line no-type-assertion/no-type-assertion
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.environments ?? {},
            } as any);
            // Mock Date.now to always return 2022-12-16T14:20:00
            Date.now = jest.fn(() => Date.parse('2022-12-16T14:20:00'));
            const { container } = getWrapper(testcase.props);
            expect(container.querySelector('.release__details-mini')?.textContent).toContain(testcase.msg);
            expect(container.querySelector('.env-group-chip-list-test')).not.toBeEmptyDOMElement();
            expect(container.querySelector('.release__details-header')?.textContent).toBe(testcase.expectedMessage);
        });
    });
});
