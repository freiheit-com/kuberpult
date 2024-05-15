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
import * as React from 'react';
import classNames from 'classnames';
import { useCallback, useRef } from 'react';
import { useTeamNames } from '../../utils/store';
import { useSearchParams } from 'react-router-dom';
import { Checkbox } from './checkbox';
import { PlainDialog } from '../dialog/ConfirmationDialog';
import { Button } from '../button';

export type DropdownProps = {
    className?: string;
    placeholder?: string;
    leadingIcon?: string;
};

export type DropdownSelectProps = {
    handleChange: (id: string | undefined) => void;
    isEmpty: (arr: string[] | undefined) => boolean;
    allTeams: string[];
    selectedTeams: string[];
};

const allTeamsId = 'all-teams';

// A dropdown allowing multiple selections
export const DropdownSelect: React.FC<DropdownSelectProps> = (props) => {
    const { handleChange, allTeams, selectedTeams } = props;

    const [open, setOpen] = React.useState(false);
    const openClose = React.useCallback(() => {
        setOpen(!open);
    }, [open, setOpen]);
    const onCancel = React.useCallback(() => {
        setOpen(false);
    }, []);

    const onChange = React.useCallback(
        (id: string) => {
            handleChange(id);
        },
        [handleChange]
    );
    const onClear = React.useCallback(() => {
        handleChange(undefined);
    }, [handleChange]);
    const onSelectAll = React.useCallback(() => {
        handleChange(allTeamsId);
    }, [handleChange]);

    const allTeamsLabel = 'Clear';
    return (
        <div className={'dropdown-container'}>
            <div className={'dropdown-arrow-container'}>
                <div className={'dropdown-arrow'}>⌄</div>
                <input
                    type="text"
                    className="dropdown-input"
                    value={selectedTeams.length === 0 ? 'Filter Teams' : '' + selectedTeams.join(', ')}
                    aria-label={'Teams'}
                    disabled={open}
                    onChange={openClose}
                    onSelect={openClose}
                    data-testid="teams-dropdown-input"
                />
            </div>
            <PlainDialog open={open} onClose={onCancel} classNames={'dropdown'} disableBackground={true} center={false}>
                <div>
                    {allTeams.map((team: string) => (
                        <div key={team}>
                            <Checkbox
                                id={team}
                                enabled={selectedTeams?.includes(team)}
                                label={team}
                                onClick={onChange}
                            />
                        </div>
                    ))}
                    <div className={'confirmation-dialog-footer'}>
                        <div className={'item'} key={'button-menu-clear'} title={'ESC also closes the dialog'}>
                            <Button
                                className="mdc-button--unelevated button-confirm"
                                label={'Select All'}
                                onClick={onSelectAll}
                                highlightEffect={false}
                            />
                        </div>
                        <div className={'item'} key={'button-menu-all'} title={'ESC also closes the dialog'}>
                            <Button
                                className="mdc-button--unelevated button-confirm"
                                label={allTeamsLabel}
                                onClick={onClear}
                                highlightEffect={false}
                            />
                        </div>
                    </div>
                </div>
            </PlainDialog>
        </div>
    );
};

export const Dropdown = (props: DropdownProps): JSX.Element => {
    const { className, placeholder, leadingIcon } = props;
    const control = useRef<HTMLDivElement>(null);
    const teams = useTeamNames();
    const [searchParams, setSearchParams] = useSearchParams();

    const allClassName = classNames(
        'mdc-select',
        'mdc-select--outlined',
        {
            'mdc-select--no-label': !placeholder,
            'mdc-select--with-leading-icon': leadingIcon,
        },
        className
    );
    const separator = ',';
    const selectedTeams = (searchParams.get('teams') || '')
        .split(separator)
        .filter((t: string) => t !== null && t !== '');

    const isEmpty = useCallback(
        (arr: string[] | undefined) => (arr ? arr.filter((val) => val !== '').length === 0 : true),
        []
    );

    const handleChange = useCallback(
        (team: string | undefined) => {
            if (team === undefined) {
                searchParams.delete('teams');
                setSearchParams(searchParams);
                return;
            }
            if (team === allTeamsId) {
                searchParams.set('teams', teams.join(separator));
                setSearchParams(searchParams);
                return;
            }

            const index = selectedTeams.indexOf(team);
            let newTeams = selectedTeams;
            if (index >= 0) {
                newTeams.splice(index, 1);
            } else {
                newTeams = selectedTeams.concat([team]);
            }
            if (newTeams.length === 0) {
                searchParams.delete('teams');
            } else {
                searchParams.set('teams', newTeams.join(separator));
            }
            setSearchParams(searchParams);
        },
        [teams, searchParams, setSearchParams, selectedTeams]
    );

    return (
        <div className={allClassName} ref={control}>
            <DropdownSelect
                handleChange={handleChange}
                isEmpty={isEmpty}
                allTeams={teams}
                selectedTeams={selectedTeams}
            />
        </div>
    );
};
