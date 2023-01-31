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
import { Priority, Release } from '../../../api/api';

describe('Release Dialog', () => {
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        rels: Release[];
        expect_message: boolean;
        data_length: number;
    }
    const data: dataT[] = [
        {
            name: 'normal release',
            props: {
                app: 'test1',
                version: 2,
                release: {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                },
                envs: [
                    {
                        name: 'prod',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 2,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                    },
                ],
            },
            rels: [],

            expect_message: true,
            data_length: 1,
        },
        {
            name: 'two envs release',
            props: {
                app: 'test1',
                version: 2,
                release: {
                    version: 2,
                    sourceMessage: 'test1',
                    sourceAuthor: 'test',
                    sourceCommitId: 'commit',
                    createdAt: new Date(2002),
                    undeployVersion: false,
                    prNumber: '#1337',
                },
                envs: [
                    {
                        name: 'prod',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 2,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                    },
                    {
                        name: 'dev',
                        locks: { envLock: { message: 'envLock', lockId: 'ui-envlock' } },
                        applications: {
                            test1: {
                                name: 'test1',
                                version: 3,
                                locks: { applock: { message: 'appLock', lockId: 'ui-applock' } },
                                queuedVersion: 0,
                                undeployVersion: false,
                            },
                        },
                        distanceToUpstream: 0,
                        priority: Priority.UPSTREAM,
                    },
                ],
            },
            rels: [],

            expect_message: true,
            data_length: 2,
        },
        {
            name: 'no release',
            props: {
                app: 'test1',
                version: -1,
                release: {} as Release,
                envs: [],
            },
            rels: [],
            expect_message: false,
            data_length: 0,
        },
    ];

    describe.each(data)(`Renders a Release Dialog`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
                environmentGroups: [
                    {
                        environmentGroupName: 'dev',
                        environments: testcase.props.envs,
                        distanceToUpstream: 2,
                    },
                ],
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
            expect(document.querySelectorAll('.env-card-data')).toHaveLength(testcase.data_length);
        });
    });

    describe.each(data)(`Renders the environment cards`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
                environmentGroups: [
                    {
                        environmentGroupName: 'dev',
                        environments: testcase.props.envs,
                        distanceToUpstream: 2,
                    },
                ],
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            expect(document.querySelector('.release-env-list')?.children).toHaveLength(testcase.props.envs.length);
        });
    });

    describe.each(data)(`Renders the environment locks`, (testcase) => {
        it(testcase.name, () => {
            // when
            UpdateOverview.set({
                applications: { [testcase.props.app as string]: { releases: testcase.rels } },
                environments: testcase.props.envs,
                environmentGroups: [
                    {
                        environmentGroupName: 'dev',
                        environments: testcase.props.envs,
                        distanceToUpstream: 2,
                    },
                ],
            } as any);
            updateReleaseDialog(testcase.props.app, testcase.props.version);
            render(<ReleaseDialog {...testcase.props} />);
            // console.info('SU DEBUG body: ', document.body.textContent);
            expect(document.body).toMatchSnapshot();
            expect(document.querySelectorAll('.release-env-group-list')).toHaveLength(1);

            testcase.props.envs.forEach((env) => {
                expect(document.querySelector('.env-locks')?.children).toHaveLength(Object.values(env.locks).length);
            });
        });
    });
});
