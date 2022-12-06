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
import { ReleaseDialog, ReleaseDialogProps } from './ReleaseDialog';
import { render } from '@testing-library/react';
import { UpdateOverview, updateReleaseDialog } from '../../utils/store';

describe('Release Card', () => {
    const getNode = (overrides: ReleaseDialogProps) => <ReleaseDialog {...overrides} />;
    const getWrapper = (overrides: ReleaseDialogProps) => render(getNode(overrides));

    const data = [
        {
            name: 'undeploy release',
            props: { app: 'test1', version: -1, release: { version: 2, sourceMessage: 'test-rel' }, envs: ['dev'] },
            rels: [{ version: 2, sourceMessage: 'test-rel' }],
            environments: {},
        },
    ];

    describe.each(data)(`Renders a Release Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.environments ?? {},
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            const { container } = getWrapper(testcase.props);

            expect(container.querySelector('.release-dialog-message')?.textContent).toContain(
                testcase.props.release.sourceMessage
            );
        });
    });
});
