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
import { ReleaseCard, ReleaseCardProps } from './ReleaseCard';
import { render } from '@testing-library/react';

const getSampleRelease = (n: string): ReleaseCardProps => ({
    title: 'test' + n,
    author: 'tester' + n + '@freiheit.com',
    hash: '12345' + n,
    createdAt: new Date(2002),
    environments: ['dev'],
});

describe('Release Card', () => {
    const getNode = (overrides: ReleaseCardProps) => <ReleaseCard {...overrides} />;
    const getWrapper = (overrides: ReleaseCardProps) => render(getNode(overrides));

    const data = [
        {
            name: 'sample release',
            rel: getSampleRelease('0'),
        },
    ];

    describe.each(data)(`Renders a`, (testcase) => {
        it(testcase.name, () => {
            const { container } = getWrapper(testcase.rel);
            expect(container.querySelector('.release__hash')?.textContent).toBe(testcase.rel.hash);
            expect(container.querySelector('.release__author')?.textContent).toContain(testcase.rel.author);
            expect(container.querySelector('.release__environments')?.textContent).toContain(
                testcase.rel.environments[0]
            );
        });
    });
});
