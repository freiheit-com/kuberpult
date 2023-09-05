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
import { render, renderHook } from '@testing-library/react';
import { Home } from './Home';
import { searchCustomFilter, UpdateOverview, useFilteredApps, useTeamNames } from '../../utils/store';
import { Spy } from 'spy4js';
import { MemoryRouter } from 'react-router-dom';
import { Application, UndeploySummary } from '../../../api/api';
import { fakeLoadEverything } from '../../../setupTests';

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
            undeploySummary: UndeploySummary.Normal,
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
        });
        fakeLoadEverything(true);
        getWrapper();

        // then apps are sorted and Service Lane is called
        expect(mock_ServiceLane.ServiceLane.getCallArgument(0, 0)).toStrictEqual({ application: sampleApps.app1 });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(1, 0)).toStrictEqual({ application: sampleApps.app2 });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(2, 0)).toStrictEqual({ application: sampleApps.app3 });
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
});

describe('Get teams from application list (useTeamNames)', () => {
    interface dataT {
        name: string;
        applications: { [key: string]: Application };
        expectedTeams: string[];
    }

    const data: dataT[] = [
        {
            name: 'right amount of teams - 4 sorted results',
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: 'test2',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedTeams: ['dummy', 'foo', 'test', 'test2'],
        },
        {
            name: "doesn't collect duplicate team names - 2 sorted results",
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedTeams: ['dummy', 'foo'],
        },
        {
            name: "doesn't collect empty team names and adds <No Team> option to dropdown - 2 sorted results",
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: '',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: '',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedTeams: ['<No Team>', 'foo', 'test'],
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ applications: testcase.applications });
            // when
            const teamNames = renderHook(() => useTeamNames()).result.current;
            expect(teamNames).toStrictEqual(testcase.expectedTeams);
        });
    });
});

describe('Get applications from selected teams (useFilteredApps)', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        applications: { [key: string]: Application };
        expectedNumOfTeams: number;
    }

    const data: dataT[] = [
        {
            name: 'gets filtered apps by team - 2 results',
            selectedTeams: ['dummy', 'foo'],
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: 'test2',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedNumOfTeams: 2,
        },
        {
            name: 'shows both applications of the selected team - 2 results',
            selectedTeams: ['dummy'],
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedNumOfTeams: 2,
        },
        {
            name: 'no teams selected (shows every application) - 4 results',
            selectedTeams: [],
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: '',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: '',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedNumOfTeams: 4,
        },
        {
            name: 'selected team has no assigned applications - 0 results',
            selectedTeams: ['thisTeamDoesntExist'],
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                    undeploySummary: UndeploySummary.Normal,
                    warnings: [],
                },
            },
            expectedNumOfTeams: 0,
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ applications: testcase.applications });
            // when
            const numOfTeams = renderHook(() => useFilteredApps(testcase.selectedTeams)).result.current.length;
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
