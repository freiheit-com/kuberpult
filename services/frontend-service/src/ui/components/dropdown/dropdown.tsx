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
import { InputLabel, MenuItem, OutlinedInput, Select } from '@material-ui/core';
import classNames from 'classnames';
import { useRef } from 'react';
import { useTeamNames } from '../../utils/store';

export type DropdownProps = {
    className?: string;
    floatingLabel?: string;
    leadingIcon?: string;
};

export const Dropdown = (props: DropdownProps) => {
    const { className, floatingLabel, leadingIcon } = props;
    const control = useRef<HTMLDivElement>(null);
    const teams = useTeamNames();
    var team = '';
    console.log(teams);
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
            <InputLabel className="mdc-floating-label">{floatingLabel}</InputLabel>
            <Select defaultValue={'All'} className="mdc-select">
                <MenuItem value={''}>All</MenuItem>
                {teams.map((el: string) => (
                    <MenuItem value={el}>{el}</MenuItem>
                ))}
            </Select>
        </div>
    );
};
