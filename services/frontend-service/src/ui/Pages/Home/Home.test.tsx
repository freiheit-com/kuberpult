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
import { render, renderHook } from '@testing-library/react';
import { Home } from './Home';
import { searchCustomFilter, UpdateOverview, useFilteredApps, useTeamNames } from '../../utils/store';
import { Spy } from 'spy4js';
import { MemoryRouter } from 'react-router-dom';
import { Application } from '../../../api/api';

const mock_ServiceLane = Spy.mockReactComponents('../../components/ServiceLane/ServiceLane', 'ServiceLane');

describe('App', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <Home />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());
    it('Renders full app', () => {
        // when
        const appBuilder = (suffix: string) => ({
            name: 'test' + suffix,
            releases: [],
            sourceRepoUrl: 'http://test' + suffix + '.com',
            team: 'example' + suffix,
        });
        UpdateOverview.set({
            environments: {},
            applications: {
                app1: appBuilder('1'),
                app3: appBuilder('3'),
                app2: appBuilder('2'),
            },
        } as any);
        getWrapper();

        // then apps are sorted and Service Lane is called
        expect(mock_ServiceLane.ServiceLane.getCallArgument(0, 0)).toStrictEqual({ application: appBuilder('1') });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(1, 0)).toStrictEqual({ application: appBuilder('2') });
        expect(mock_ServiceLane.ServiceLane.getCallArgument(2, 0)).toStrictEqual({ application: appBuilder('3') });
    });
});

describe('Application Filter', () => {
    interface dataT {
        name: string;
        query: string;
        applications: string[];
        expect: (nrLocks: number) => void;
    }

    const data: dataT[] = [
        {
            name: 'filter applications - 1 result',
            applications: ['dummy', 'test', 'test2', 'foo'],
            query: 'dummy',
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(1),
        },
        {
            name: 'filter applications - 0 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'foo',
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(0),
        },
        {
            name: 'filter applications - 2 results',
            applications: ['dummy', 'test', 'test2'],
            query: 'test',
            expect: (nrLocks) => expect(nrLocks).toStrictEqual(2),
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const nrLocks = testcase.applications.filter((val) => searchCustomFilter(testcase.query, val)).length;
            testcase.expect(nrLocks);
        });
    });
});

describe('Get teams from application list (useTeamNames)', () => {
    interface dataT {
        name: string;
        applications: { [key: string]: Application };
        expect: (teamNames: string[]) => void;
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
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: 'test2',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(['dummy', 'foo', 'test', 'test2']),
        },
        {
            name: "doesn't collect duplicate team names - 2 sorted results",
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'dummy',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(['dummy', 'foo']),
        },
        {
            name: "doesn't collect empty team names - 2 sorted results",
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: '',
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: '',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(['foo', 'test']),
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ applications: testcase.applications, environments: {} });
            // when
            const teamNames = renderHook(() => useTeamNames()).result.current;
            testcase.expect(teamNames);
        });
    });
});

describe('Get applications from selected teams (useFilteredApps)', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        applications: { [key: string]: Application };
        expect: (teamNames: number) => void;
    }

    const data: dataT[] = [
        {
            name: 'gets every app - 2 results',
            selectedTeams: ['dummy', 'foo'],
            applications: {
                foo: {
                    name: 'foo',
                    releases: [],
                    sourceRepoUrl: 'http://foo.com',
                    team: 'dummy',
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: 'test2',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(2),
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
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'dummy',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(2),
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
                },
                bar: {
                    name: 'bar',
                    releases: [],
                    sourceRepoUrl: 'http://bar.com',
                    team: 'test',
                },
                example: {
                    name: 'example',
                    releases: [],
                    sourceRepoUrl: 'http://example.com',
                    team: '',
                },
                team: {
                    name: 'team',
                    releases: [],
                    sourceRepoUrl: 'http://team.com',
                    team: 'foo',
                },
            },
            expect: (teamNames) => expect(teamNames).toStrictEqual(4),
        },
    ];

    describe.each(data)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ applications: testcase.applications, environments: {} });
            // when
            const teamNames = renderHook(() => useFilteredApps(testcase.selectedTeams)).result.current.length;
            testcase.expect(teamNames);
        });
    });
});
