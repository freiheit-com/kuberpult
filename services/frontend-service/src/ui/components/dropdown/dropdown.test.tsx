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

import { DropdownSelect } from './dropdown';
import { getByTestId, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import '@testing-library/user-event';
import { UpdateOverview } from '../../utils/store';

describe('Dropdown', () => {
    const getNode = (overrides?: {}): JSX.Element | any => {
        const defaultProps: any = {
            children: null,
        };
        // given
        return (
            <div>
                <MemoryRouter>
                    <DropdownSelect {...defaultProps} {...overrides} />
                </MemoryRouter>
            </div>
        );
    };

    const getWrapper = (
        overrides?: {
            className: string;
            applications: any;
            handleChange: (event: any) => void;
            isEmpty: (arr: string[] | undefined) => boolean;
            floatingLabel: string;
            teams: string[];
            selectedTeams: string[];
        },
        entries?: string[]
    ) => render(getNode(overrides));

    interface dataT {
        name: string;
        className: string;
        handleChange: (event: any) => void;
        isEmpty: (arr: string[] | undefined) => boolean;
        floatingLabel: string;
        teams: string[];
        selectedTeams: string[];
        applications: any;
        expect: (container: HTMLElement) => void;
    }

    const data: dataT[] = [
        {
            name: 'Get label when no teams are selected',
            className: 'top-app-bar-search-field',
            handleChange: (event: any) => {},
            isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
            floatingLabel: 'Teams',
            teams: ['example', 'bar'],
            selectedTeams: [],
            applications: {},
            expect: (container) => {
                const label = screen.getByTestId('teams-dropdown-label');
                expect(label).toHaveAttribute('data-shrink', 'false');
                expect(label).toHaveTextContent(/^Teams$/);
            },
        },
        {
            name: 'Get value after selecting a team',
            className: 'top-app-bar-search-field',
            handleChange: (event: any) => {},
            isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
            floatingLabel: 'Teams',
            teams: ['example', 'bar'],
            selectedTeams: ['example'],
            applications: {},
            expect: (container) => {
                const label = screen.getByTestId('teams-dropdown-label');
                expect(label).toHaveTextContent(/^Teams$/);
                expect(
                    getByTestId(container, 'teams-dropdown-select').getElementsByClassName('MuiSelect-select')[0]
                ).toHaveTextContent(/^example$/);
            },
        },
        {
            name: 'Get value after selecting multiple teams',
            className: 'top-app-bar-search-field',
            handleChange: (event: any) => {},
            isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
            floatingLabel: 'Teams',
            teams: ['example', 'bar'],
            selectedTeams: ['example', 'bar'],
            applications: {},
            expect: (container) => {
                const label = screen.getByTestId('teams-dropdown-label');
                expect(label).toHaveTextContent(/^Teams$/);
                expect(
                    getByTestId(container, 'teams-dropdown-select').getElementsByClassName('MuiSelect-select')[0]
                ).toHaveTextContent(/^example, bar$/);
            },
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            // given
            UpdateOverview.set({ applications: testcase.applications, environments: {} });
            const { isEmpty, handleChange, className, floatingLabel, teams, selectedTeams, applications } = testcase;
            // when
            const { container } = getWrapper({
                isEmpty,
                handleChange,
                floatingLabel,
                className,
                teams,
                selectedTeams,
                applications,
            });
            // then
            testcase.expect(container);
        });
    });
});
