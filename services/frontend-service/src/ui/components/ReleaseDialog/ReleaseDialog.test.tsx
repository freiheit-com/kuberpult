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
import { EnvironmentListItem, ReleaseDialog, ReleaseDialogProps } from './ReleaseDialog';
import { render } from '@testing-library/react';
import { UpdateOverview, updateReleaseDialog, UpdateSidebar } from '../../utils/store';
import { Priority, Release } from '../../../api/api';
import { Spy } from 'spy4js';
import { SideBar } from '../SideBar/SideBar';

const mock_getFormattedReleaseDate = Spy.mockModule('../ReleaseCard/ReleaseCard', 'getFormattedReleaseDate');

describe('Release Dialog', () => {
    interface dataT {
        name: string;
        props: ReleaseDialogProps;
        rels: Release[];
        expect_message: boolean;
        expect_queues: number;
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
            expect_queues: 0,
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
                                queuedVersion: 666,
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
            expect_queues: 1,
            data_length: 3,
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
            expect_queues: 0,
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
            expect(document.querySelectorAll('.env-card-data-queue')).toHaveLength(testcase.expect_queues);
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
            // given
            mock_getFormattedReleaseDate.getFormattedReleaseDate.returns(<div>some formatted date</div>);
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
            expect(document.body).toMatchSnapshot();
            expect(document.querySelectorAll('.release-env-group-list')).toHaveLength(1);

            testcase.props.envs.forEach((env) => {
                expect(document.querySelector('.env-locks')?.children).toHaveLength(Object.values(env.locks).length);
            });
        });
    });

    describe.each(data)(`Renders the queuedVersion`, (testcase) => {
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
            expect(document.querySelectorAll('.env-card-data-queue')).toHaveLength(testcase.expect_queues);
        });
    });

    describe(`Test automatic cart opening`, () => {
        const testcase = data[0];
        it('Test using direct call to open function', () => {
            if (UpdateSidebar.get().shown === true) {
                UpdateSidebar.set({ shown: false });
            }
            UpdateSidebar.set({ shown: true });
            expect(UpdateSidebar.get().shown).toBeTruthy();
        });
        it('Test using deploy button click simulation', () => {
            if (UpdateSidebar.get().shown === true) {
                UpdateSidebar.set({ shown: false });
            }
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

            render(<ReleaseDialog {...testcase.props} />);
            render(
                <EnvironmentListItem
                    env={testcase.props.envs[0]}
                    app={testcase.props.app}
                    queuedVersion={0}
                    release={testcase.props.release}
                />
            );
            render(<SideBar />);
            const result = document.querySelector('.env-card-deploy-btn');
            result?.click();
            expect(UpdateSidebar.get().shown).toBeTruthy();
        });
        it('Test using add lock button click simulation', () => {
            if (UpdateSidebar.get().shown === true) {
                UpdateSidebar.set({ shown: false });
            }
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

            render(<ReleaseDialog {...testcase.props} />);
            render(
                <EnvironmentListItem
                    env={testcase.props.envs[0]}
                    app={testcase.props.app}
                    queuedVersion={0}
                    release={testcase.props.release}
                />
            );
            render(<SideBar />);
            const result = document.querySelector('.env-card-add-lock-btn');
            result?.click();
            expect(UpdateSidebar.get().shown).toBeTruthy();
        });
    });
});
