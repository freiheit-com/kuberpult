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
import { TeamsFilterDropdownSelect, DropdownSelectProps } from './dropdown';
import { getByTestId, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

const getNode = (overrides?: {}): JSX.Element | any => {
    const defaultProps: any = {
        children: null,
    };
    // given
    return (
        <div>
            <MemoryRouter>
                <TeamsFilterDropdownSelect {...defaultProps} {...overrides} />
            </MemoryRouter>
        </div>
    );
};

const getWrapper = (overrides?: DropdownSelectProps, entries?: string[]) => render(getNode(overrides));

describe('Dropdown label', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        expectedLabel: string;
    }

    const data: dataT[] = [
        {
            name: 'Get label when no teams are selected',
            selectedTeams: [],
            expectedLabel: 'Filter Teams',
        },
        {
            name: 'Get label when a team is selected',
            selectedTeams: ['foo', 'bar'],
            expectedLabel: 'foo, bar',
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            // given
            // when
            const { container } = getWrapper({
                isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
                handleChange: (event: any) => {},
                allTeams: ['Test', 'foo', 'bar'],
                selectedTeams: testcase.selectedTeams,
            });
            // then
            expect(getByTestId(container, 'teams-dropdown-input')).toHaveAttribute('value', testcase.expectedLabel);
        });
    });
});

describe('Dropdown dropdown text with selected teams', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        expectedTeamsText: string;
    }

    const data: dataT[] = [
        {
            name: 'Get value after selecting a team',
            selectedTeams: ['example'],
            expectedTeamsText: 'example',
        },
        {
            name: 'Get value after selecting multiple teams',
            selectedTeams: ['example', 'bar'],
            expectedTeamsText: 'example, bar',
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            // given
            const { selectedTeams } = testcase;
            // when
            const { container } = getWrapper({
                isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
                handleChange: (event: any) => {},
                allTeams: ['example', 'bar'],
                selectedTeams,
            });
            // then
            expect(getByTestId(container, 'teams-dropdown-input')).toHaveAttribute('value', testcase.expectedTeamsText);
        });
    });
});
