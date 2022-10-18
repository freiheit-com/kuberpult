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
import { Checkbox, FormControl, InputLabel, MenuItem, Select } from '@material-ui/core';
import classNames from 'classnames';
import { useCallback, useRef } from 'react';
import { useTeamNames } from '../../utils/store';
import { useSearchParams } from 'react-router-dom';

export type DropdownProps = {
    className?: string;
    floatingLabel?: string;
    leadingIcon?: string;
};

export const DropdownSelect: React.FC<{
    handleChange: (event: any) => void;
    isEmpty: (arr: string[] | undefined) => boolean;
    floatingLabel: string | undefined;
    teams: string[];
    selectedTeams: string[];
}> = (props) => {
    const { handleChange, isEmpty, floatingLabel, teams, selectedTeams } = props;
    const renderValue = useCallback((selected: string[]) => selected.join(', '), []);

    return (
        <FormControl variant="outlined" fullWidth data-testid="teams-dropdown-formcontrol">
            <InputLabel htmlFor="teams" id="teams" shrink={!isEmpty(selectedTeams)} data-testid="teams-dropdown-label">
                {floatingLabel}
            </InputLabel>
            <Select
                data-testid="teams-dropdown-select"
                labelId="teams"
                multiple={true}
                renderValue={renderValue}
                value={selectedTeams}
                onChange={handleChange}
                className={'mdc-select ' + (!isEmpty(selectedTeams) ? '' : 'remove-space')}
                label={!isEmpty(selectedTeams) ? floatingLabel : ''}
                variant="outlined">
                <MenuItem data-testid="clear-option" key={''} value={''}>
                    Clear
                </MenuItem>
                {teams.map((team: string) => (
                    <MenuItem key={team} value={team}>
                        <Checkbox checked={selectedTeams?.includes(team)}></Checkbox>
                        {team}
                    </MenuItem>
                ))}
            </Select>
        </FormControl>
    );
};

export const Dropdown = (props: DropdownProps) => {
    const { className, floatingLabel, leadingIcon } = props;
    const control = useRef<HTMLDivElement>(null);
    const teams = useTeamNames();
    const [searchParams, setSearchParams] = useSearchParams();

    const allClassName = classNames(
        'mdc-select',
        'mdc-select--outlined',
        {
            'mdc-select--no-label': !floatingLabel,
            'mdc-select--with-leading-icon': leadingIcon,
        },
        className
    );
    const selectedTeams = (searchParams.get('teams') || '').split(',');

    const isEmpty = useCallback(
        (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
        []
    );

    const handleChange = useCallback(
        (event: any) => {
            if (event.target.value.indexOf('') > 0 || event.target.value.length === 0) searchParams.delete('teams');
            else
                searchParams.set(
                    'teams',
                    event.target.value.filter((team: string) => team !== '')
                );
            setSearchParams(searchParams);
        },
        [searchParams, setSearchParams]
    );

    return (
        <div className={allClassName} ref={control}>
            <DropdownSelect
                handleChange={handleChange}
                isEmpty={isEmpty}
                floatingLabel={floatingLabel}
                teams={teams}
                selectedTeams={selectedTeams}
            />
        </div>
    );
};
