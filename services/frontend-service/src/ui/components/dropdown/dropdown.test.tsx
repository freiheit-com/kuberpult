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
                <DropdownSelect {...defaultProps} {...overrides} />
            </MemoryRouter>
        </div>
    );
};

const getWrapper = (
    overrides?: {
        handleChange: (event: any) => void;
        isEmpty: (arr: string[] | undefined) => boolean;
        floatingLabel: string;
        teams: string[];
        selectedTeams: string[];
    },
    entries?: string[]
) => render(getNode(overrides));

describe('Dropdown label', () => {
    interface dataT {
        name: string;
        floatingLabel: string;
        expectedLabel: RegExp;
    }

    const data: dataT[] = [
        {
            name: 'Get label when no teams are selected',
            floatingLabel: 'Teams',
            expectedLabel: /^Teams$/,
        },
        {
            name: 'Get label when no teams are selected',
            floatingLabel: 'Test',
            expectedLabel: /^Test$/,
        },
    ];

    describe.each(data)(`Renders a navigation item with selected`, (testcase) => {
        it(testcase.name, () => {
            // given
            const { floatingLabel } = testcase;
            // when
            const { container } = getWrapper({
                isEmpty: (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
                handleChange: (event: any) => {},
                floatingLabel: floatingLabel,
                teams: ['example', 'bar'],
                selectedTeams: [],
            });
            // then
            expect(getByTestId(container, 'teams-dropdown-label')).toHaveTextContent(testcase.expectedLabel);
        });
    });
});

describe('Dropdown shrink label', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        expectedShrink: string;
    }

    const data: dataT[] = [
        {
            name: 'Check if label is not shrunk when there are no teams selected',
            selectedTeams: [],
            expectedShrink: 'false',
        },
        {
            name: 'Check if label is shrunk when there are teams selected',
            selectedTeams: ['example', 'bar'],
            expectedShrink: 'true',
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
                floatingLabel: 'Teams',
                teams: ['example', 'bar'],
                selectedTeams,
            });
            // then
            expect(getByTestId(container, 'teams-dropdown-label')).toHaveAttribute(
                'data-shrink',
                testcase.expectedShrink
            );
        });
    });
});

describe('Dropdown shrink label', () => {
    interface dataT {
        name: string;
        selectedTeams: string[];
        expectedTeamsText: RegExp;
    }

    const data: dataT[] = [
        {
            name: 'Get value after selecting a team',
            selectedTeams: ['example'],
            expectedTeamsText: /^example$/,
        },
        {
            name: 'Get value after selecting multiple teams',
            selectedTeams: ['example', 'bar'],
            expectedTeamsText: /^example, bar$/,
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
                floatingLabel: 'Teams',
                teams: ['example', 'bar'],
                selectedTeams,
            });
            // then
            expect(
                getByTestId(container, 'teams-dropdown-select').getElementsByClassName('MuiSelect-select')[0]
            ).toHaveTextContent(testcase.expectedTeamsText);
            // testcase.expect(container);
        });
    });
});
