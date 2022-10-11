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

export const Dropdown = (props: DropdownProps) => {
    const { className, floatingLabel, leadingIcon } = props;
    const control = useRef<HTMLDivElement>(null);
    const teams = useTeamNames();
    const [searchParams, setSearchParams] = useSearchParams();

    const isEmpty = (arr: string[]) => arr.length === 0;
    const handleChange = useCallback(
        (event: any) => {
            if (event.target.value.includes('')) setSearchParams({ teams: [] });
            else setSearchParams({ teams: event.target.value });
        },
        [setSearchParams]
    );

    const renderValue = useCallback((selected: string[]) => selected.join(', '), []);

    const allClassName = classNames(
        'mdc-select',
        'mdc-select--outlined',
        {
            'mdc-select--no-label': !floatingLabel,
            'mdc-select--with-leading-icon': leadingIcon,
        },
        className
    );

    return (
        <div className={allClassName} ref={control}>
            <FormControl variant="outlined" fullWidth>
                <InputLabel htmlFor="teams" id="teams" shrink={!isEmpty(searchParams.getAll('teams')) ? true : false}>
                    {floatingLabel}
                </InputLabel>
                <Select
                    labelId="teams"
                    multiple={true}
                    renderValue={renderValue}
                    value={searchParams.getAll('teams')}
                    onChange={handleChange}
                    className={'mdc-select ' + (!isEmpty(searchParams.getAll('teams')) ? '' : 'remove-space')}
                    label={!isEmpty(searchParams.getAll('teams')) ? floatingLabel : ''}
                    variant="outlined">
                    <MenuItem key={''} value={''}>
                        Clear
                    </MenuItem>
                    {teams.map((team: string) => (
                        <MenuItem key={team} value={team}>
                            <Checkbox checked={searchParams.getAll('teams').includes(team)}></Checkbox>
                            {team}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>
        </div>
    );
};
