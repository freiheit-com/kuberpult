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
import { UpdateOverview, updateReleaseDialog } from '../../utils/store';
import { Application, Environment, Release } from '../../../api/api';
import { ReleaseDialog } from './ReleaseDialog';
import { render } from '@testing-library/react';

describe('Release Dialog', () => {
    interface dataT {
        name: string;
        props: { app: { [key: string]: Application }; version: number };
        rels: Release[];
        environments: { [key: string]: Environment };
        hasRelease: boolean;
    }
    const data: dataT[] = [
        {
            name: 'normal release',
            props: {
                app: {
                    test1: {
                        name: 'test1',
                        releases: [
                            {
                                version: 2,
                                sourceMessage: 'test1',
                                sourceAuthor: 'test',
                                sourceCommitId: 'commit',
                                createdAt: new Date(2002),
                                undeployVersion: false,
                                prNumber: '#1337',
                            },
                        ],
                        sourceRepoUrl: 'test.com',
                        team: 'test',
                    },
                },
                version: 2,
            },
            rels: [],
            environments: {
                prod: {
                    name: 'prod',
                    locks: {},
                    applications: {
                        test1: {
                            name: 'test1',
                            version: 2,
                            locks: { testapplock: { message: 'testapplock', lockId: 'ui-v2-testapplock' } },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            hasRelease: false,
        },
        {
            name: 'no release',
            props: {
                app: { test1: { name: 'test1', releases: [], sourceRepoUrl: 'test.com', team: 'tesst' } },
                version: -1,
            },
            rels: [],
            environments: {
                prod: {
                    name: 'prod',
                    locks: { testlock2: { message: 'testlock2', lockId: 'ui-v2-testlock2' } },
                    applications: {
                        test2: {
                            name: 'test2',
                            version: 2,
                            locks: { testapplock2: { message: 'testapplock2', lockId: 'ui-v2-testapplock2' } },
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            hasRelease: false,
        },
    ];

    describe.each(data)(`Renders a Release Message`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: testcase.props.app,
                environments: testcase.environments,
            });
            const app = Object.values(testcase.props.app)[0];
            updateReleaseDialog(app, testcase.props.version);
            render(<ReleaseDialog app={app} version={testcase.props.version} />);
            if (testcase.hasRelease) {
                expect(document.querySelector('.release-dialog-message')?.textContent).toContain(
                    app.releases[0]?.sourceMessage
                );
            } else {
                expect(document.querySelector('.release-dialog-message') === undefined);
            }
        });
    });

    describe.each(data)(`Renders the Release Author`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: testcase.props.app,
                environments: testcase.environments,
            });
            const app = Object.values(testcase.props.app)[0];
            updateReleaseDialog(app, testcase.props.version);
            render(<ReleaseDialog app={app} version={testcase.props.version} />);
            if (testcase.hasRelease) {
                expect(document.querySelector('.release-dialog-author')?.textContent).toContain(
                    app.releases[0]?.sourceAuthor
                );
            } else {
                expect(document.querySelector('.release-dialog-author') === undefined);
            }
        });
    });

    describe.each(data)(`Renders all the envs`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: testcase.props.app,
                environments: testcase.environments,
            });
            const app = Object.values(testcase.props.app)[0];
            updateReleaseDialog(app, testcase.props.version);
            render(<ReleaseDialog app={app} version={testcase.props.version} />);
            expect(document.querySelector('env-card')?.children).toHaveLength(
                Object.values(testcase.environments).length
            );
        });
    });
});
