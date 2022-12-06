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
import { ReleaseDialog } from './ReleaseDialog';
import { render } from '@testing-library/react';
import { UpdateOverview, updateReleaseDialog } from '../../utils/store';

describe('Release Dialog', () => {
    const data = [
        {
            name: 'normal release',
            props: {
                app: 'test1',
                version: 2,
                release: {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuhor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                },
                envs: [],
            },
            rels: [],
            environments: {},
            expect_message: true,
        },
        {
            name: 'no release',
            props: {
                app: 'test1',
                version: -1,
                release: {},
                envs: [],
            },
            rels: [],
            environments: {},
            expect_message: false,
        },
    ];

    describe.each(data)(`Renders a Release Dialog`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.environments ?? {},
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            if (testcase.expect_message) {
                expect(document.querySelector('.release-dialog-message')?.textContent).toContain(
                    testcase.props.release.sourceMessage
                );
            } else {
                expect(document.querySelector('.release-dialog-message') === undefined);
            }
        });
    });
});
