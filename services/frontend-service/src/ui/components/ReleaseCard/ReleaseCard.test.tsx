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
import { ReleaseCard, ReleaseCardProps } from './ReleaseCard';
import { render } from '@testing-library/react';
import { UpdateOverview } from '../../utils/store';

describe('Release Card', () => {
    const getNode = (overrides: ReleaseCardProps) => <ReleaseCard {...overrides} />;
    const getWrapper = (overrides: ReleaseCardProps) => render(getNode(overrides));

    const data = [
        {
            name: 'using a sample release - useRelease hook',
            props: { app: 'test1', version: 2 },
            rels: [{ version: 2, sourceMessage: 'test-rel' }],
        },
        {
            name: 'using a sample undeploy release - useRelease hook',
            props: { app: 'test2', version: -1 },
            rels: [{ undeployVersion: true, sourceMessage: 'test-rel' }],
        },
        {
            name: 'using a full release - component test',
            props: { app: 'test2', version: 2 },
            rels: [
                {
                    undeployVersion: false,
                    version: 2,
                    sourceMessage: 'test-rel',
                    sourceCommitId: '12s3',
                    sourceAuthor: 'test-author',
                    createdAt: new Date(2002),
                } as any,
            ],
        },
        {
            name: 'using a deployed release - useDeployedAt test',
            props: { app: 'test2', version: 2 },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                } as any,
            ],
            environments: {
                foo: {
                    name: 'foo',
                    applications: {
                        test2: {
                            version: 2,
                        },
                    },
                },
            },
        },
        {
            name: 'using an undeployed release - useDeployedAt test',
            props: { app: 'test2', version: 2 },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                } as any,
            ],
            environments: {
                undeployed: {
                    name: 'undeployed',
                    applications: {
                        test2: {
                            version: 3,
                        },
                    },
                },
            },
        },
        {
            name: 'using another environment - useDeployedAt test',
            props: { app: 'test2', version: 2 },
            rels: [
                {
                    version: 2,
                    sourceMessage: 'test-rel',
                } as any,
            ],
            environments: {
                other: {
                    name: 'other',
                    applications: {
                        test3: {
                            version: 3,
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
            const { container } = getWrapper(testcase.props);

            expect(container.querySelector('.release__title')?.textContent).toContain(testcase.rels[0].sourceMessage);

            if (testcase.rels[0].sourceCommitId) {
                expect(container.querySelector('.release__hash')?.textContent).toContain(
                    testcase.rels[0].sourceCommitId
                );
            }
            if (testcase.rels[0].createdAt) {
                expect(container.querySelector('.release__metadata')?.textContent).toContain(
                    (testcase.rels[0].createdAt as Date).toLocaleDateString()
                );
            }
            expect(container.querySelector('.env-group-chip-list-test')).not.toBeEmptyDOMElement();
        });
    });
});
