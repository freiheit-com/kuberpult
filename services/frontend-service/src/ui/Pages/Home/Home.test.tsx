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

Copyright freiheit.com*/
import { render, renderHook } from '@testing-library/react';
import { Home } from './Home';
import {
    searchCustomFilter,
    updateAppDetails,
    UpdateOverview,
    useApplicationsFilteredAndSorted,
    useTeamNames,
} from '../../utils/store';
import { Spy } from 'spy4js';
import { MemoryRouter } from 'react-router-dom';
import { Application, GetAppDetailsResponse, GetOverviewResponse, UndeploySummary } from '../../../api/api';
import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';

const mock_ServiceLane = Spy.mockReactComponents('../../components/ServiceLane/ServiceLane', 'ServiceLane');

describe('App', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <Home />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());
    it('Renders full app', () => {
        const buildTestApp = (suffix: string): Application => ({
            name: `test${suffix}`,
            releases: [],
            sourceRepoUrl: `http://test${suffix}.com`,
            team: `team${suffix}`,
            undeploySummary: UndeploySummary.NORMAL,
            warnings: [],
        });
        // when
        const sampleApps = {
            app1: buildTestApp('1'),
            app2: buildTestApp('2'),
            app3: buildTestApp('3'),
        };
        UpdateOverview.set({
            applications: sampleApps,
            lightweightApps: [
                {
                    name: sampleApps.app1.name,
                    team: sampleApps.app1.team,
                },
                {
                    name: sampleApps.app2.name,
                    team: sampleApps.app2.team,
                },
                {
                    name: sampleApps.app3.name,
                    team: sampleApps.app3.team,
                },
            ],
        });
        updateAppDetails.set({
            [sampleApps.app1.name]: {
                application: sampleApps.app1,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
            [sampleApps.app2.name]: {
                application: sampleApps.app2,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
            [sampleApps.app2.name]: {
                application: sampleApps.app2,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
        });
        fakeLoadEverything(true);
        getWrapper();

        // then apps are sorted and Service Lane is called
        expect(mock_ServiceLane.ServiceLane.getCallArgument(0, 0)).toStrictEqual({
            application: { name: sampleApps.app1.name, team: sampleApps.app1.team },
            hideMinors: false,
        });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(1, 0)).toStrictEqual({
            application: { name: sampleApps.app2.name, team: sampleApps.app2.team },
            hideMinors: false,
        });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(2, 0)).toStrictEqual({
            application: { name: sampleApps.app3.name, team: sampleApps.app3.team },
            hideMinors: false,
        });
    });
    it('Renders Spinner', () => {
        // given
        UpdateOverview.set({
            loaded: false,
        });
        // when
        const { container } = getWrapper();
        // then
        expect(container.getElementsByClassName('spinner')).toHaveLength(1);
    });
    it('Renders login page if Dex enabled', () => {
        fakeLoadEverything(true);
        enableDexAuth(false);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('environment_name')[0]).toHaveTextContent('Log in to Dex');
    });
    it('Renders page if Dex enabled and valid token', () => {
        const buildTestApp = (suffix: string): Application => ({
            name: `test${suffix}`,
            releases: [],
            sourceRepoUrl: `http://test${suffix}.com`,
            team: `team${suffix}`,
            undeploySummary: UndeploySummary.NORMAL,
            warnings: [],
        });
        // when
        const sampleApps = {
            app1: buildTestApp('1'),
            app2: buildTestApp('2'),
            app3: buildTestApp('3'),
        };
        UpdateOverview.set({
            applications: sampleApps,
            lightweightApps: [
                {
                    name: sampleApps.app1.name,
                    team: sampleApps.app1.team,
                },
                {
                    name: sampleApps.app2.name,
                    team: sampleApps.app2.team,
                },
                {
                    name: sampleApps.app3.name,
                    team: sampleApps.app3.team,
                },
            ],
        });
        updateAppDetails.set({
            [sampleApps.app1.name]: {
                application: sampleApps.app1,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
            [sampleApps.app2.name]: {
                application: sampleApps.app2,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
            [sampleApps.app2.name]: {
                application: sampleApps.app2,
                deployments: {},
                appLocks: {},
                teamLocks: {},
            },
        });
        fakeLoadEverything(true);
        enableDexAuth(true);
        getWrapper();

        // then apps are sorted and Service Lane is called
        expect(mock_ServiceLane.ServiceLane.getCallArgument(0, 0)).toStrictEqual({
            application: { name: sampleApps.app1.name, team: sampleApps.app1.team },
            hideMinors: false,
        });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(1, 0)).toStrictEqual({
            application: { name: sampleApps.app2.name, team: sampleApps.app2.team },
            hideMinors: false,
        });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(2, 0)).toStrictEqual({
            application: { name: sampleApps.app3.name, team: sampleApps.app3.team },
            hideMinors: false,
        });
    });
});

describe('Get teams from application list (useTeamNames)', () => {
    interface dataT {
        name: string;
        appDetails: { [key: string]: GetAppDetailsResponse };
        overview: GetOverviewResponse;
        expectedTeams: string[];
    }

    const data: dataT[] = [
        {
            name: 'right amount of teams - 4 sorted results',
            applications: {},
            overview: {
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'test',
                    },
                    {
                        name: 'example',
                        team: 'test2',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                ],
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                example: {
                    application: {
                        name: 'example',
                        releases: [],
                        sourceRepoUrl: 'http://example.com',
                        team: 'test2',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedTeams: ['dummy', 'foo', 'test', 'test2'],
        },
        {
            name: "doesn't collect duplicate team names - 2 sorted results",
            overview: {
                applications: {},
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'dummy',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                ],
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedTeams: ['dummy', 'foo'],
        },
        {
            name: "doesn't collect empty team names and adds <No Team> option to dropdown - 2 sorted results",
            overview: {
                applications: {},
                lightweightApps: [
                    {
                        name: 'foo',
                        team: '',
                    },
                    {
                        name: 'bar',
                        team: 'test',
                    },
                    {
                        name: 'example',
                        team: '',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                ],
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: '',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                example: {
                    application: {
                        name: 'example',
                        releases: [],
                        sourceRepoUrl: 'http://example.com',
                        team: '',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedTeams: ['<No Team>', 'foo', 'test'],
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set(testcase.overview);
            UpdateOverview.set(testcase.appDetails);
            // when
            const teamNames = renderHook(() => useTeamNames()).result.current;
            expect(teamNames).toStrictEqual(testcase.expectedTeams);
        });
    });
});

describe('Get applications from selected teams (useApplicationsFilteredAndSorted)', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        Overview: GetOverviewResponse;
        expectedNumOfTeams: number;
        appDetails: { [key: string]: GetAppDetailsResponse };
    }

    const data: dataT[] = [
        {
            name: 'gets filtered apps by team - 2 results',
            selectedTeams: ['dummy', 'foo'],
            Overview: {
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'test',
                    },
                    {
                        name: 'example',
                        team: 'test2',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                ],
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                example: {
                    application: {
                        name: 'example',
                        releases: [],
                        sourceRepoUrl: 'http://example.com',
                        team: 'test2',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedNumOfTeams: 2,
        },
        {
            name: 'shows both applications of the selected team - 2 results',
            selectedTeams: ['dummy'],
            Overview: {
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'dummy',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                ],
            },
            expectedNumOfTeams: 2,
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
        },
        {
            name: 'no teams selected (shows every application) - 4 results',
            selectedTeams: [],
            Overview: {
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'test',
                    },
                    {
                        name: 'team',
                        team: 'foo',
                    },
                    {
                        name: 'example',
                        team: 'test2',
                    },
                ],
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                team: {
                    application: {
                        name: 'team',
                        releases: [],
                        sourceRepoUrl: 'http://team.com',
                        team: 'foo',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedNumOfTeams: 4,
        },
        {
            name: 'selected team has no assigned applications - 0 results',
            selectedTeams: ['thisTeamDoesntExist'],
            Overview: {
                environmentGroups: [],
                gitRevision: '',
                branch: '',
                manifestRepoUrl: '',
                lightweightApps: [
                    {
                        name: 'foo',
                        team: 'dummy',
                    },
                    {
                        name: 'bar',
                        team: 'test',
                    },
                ],
            },
            appDetails: {
                foo: {
                    application: {
                        name: 'foo',
                        releases: [],
                        sourceRepoUrl: 'http://foo.com',
                        team: 'dummy',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
                bar: {
                    application: {
                        name: 'bar',
                        releases: [],
                        sourceRepoUrl: 'http://bar.com',
                        team: 'test',
                        undeploySummary: UndeploySummary.NORMAL,
                        warnings: [],
                    },
                    appLocks: {},
                    teamLocks: {},
                    deployments: {},
                },
            },
            expectedNumOfTeams: 0,
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set(testcase.Overview);
            updateAppDetails.set(testcase.appDetails);
            // when
            const numOfTeams = renderHook(() => useApplicationsFilteredAndSorted(testcase.selectedTeams, false, ''))
                .result.current.length;
            expect(numOfTeams).toStrictEqual(testcase.expectedNumOfTeams);
        });
    });
});

describe('Application Filter', () => {
    interface dataT {
        name: string;
        query: string;
        applications: string[];
        expectedLocks: number;
    }

    const data: dataT[] = [
        {
            name: 'filter applications - 1 result',
            applications: ['dummy', 'test', 'test2', 'foo'],
            query: 'dummy',
            expectedLocks: 1,
        },
        {
            name: 'filter applications - 0 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'foo',
            expectedLocks: 0,
        },
        {
            name: 'filter applications - 2 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'test',
            expectedLocks: 2,
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const nrLocks = testcase.applications.filter((val) => searchCustomFilter(testcase.query, val)).length;
            expect(nrLocks).toStrictEqual(testcase.expectedLocks);
        });
    });
});
