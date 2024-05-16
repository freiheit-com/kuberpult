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
import { useState } from 'react';
import { Checkbox } from '../dropdown/checkbox';
import { ConfirmationDialog } from '../dialog/ConfirmationDialog';
import { showSnackbarError } from '../../utils/store';

export type TeamSelectionDialogProps = {
    teams: string[];
    environment: string;
    onSubmit: (selectedTeams: string[]) => void;
    onCancel: () => void;
    open: boolean;
    teamSelectionDialog: boolean; // false if release train dialog
};

export const TeamSelectionDialog: React.FC<TeamSelectionDialogProps> = (props) => {
    const [selectedTeams, setSelectedTeams] = useState<string[]>([]);

    const onConfirm = React.useCallback(() => {
        if (selectedTeams.length < 1) {
            showSnackbarError('There needs to be at least one team selected to perform this action');
            return;
        }
        props.onSubmit(selectedTeams);
        setSelectedTeams([]);
    }, [props, selectedTeams]);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedTeams([]);
    }, [props]);

    const addTeam = React.useCallback(
        (team: string) => {
            const newTeam = team;
            const indexOf = selectedTeams.indexOf(newTeam);
            if (indexOf >= 0) {
                const copy = selectedTeams.concat();
                copy.splice(indexOf, 1);
                setSelectedTeams(copy);
            } else if (!props.teamSelectionDialog) {
                setSelectedTeams([newTeam]);
            } else {
                setSelectedTeams(selectedTeams.concat(newTeam));
            }
        },
        [props.teamSelectionDialog, selectedTeams]
    );

    return (
        <ConfirmationDialog
            classNames={'env-selection-dialog'}
            onConfirm={onConfirm}
            onCancel={onCancel}
            open={props.open}
            headerLabel={'Select all teams for lock:'}
            confirmLabel={'Select Teams'}>
            {props.teams.length > 0 ? (
                <div className="envs-dropdown-select">
                    {props.teams.map((team: string, index: number) => {
                        const enabled = selectedTeams.includes(team);
                        // @ts-ignore
                        return (
                            <div key={team}>
                                <Checkbox
                                    enabled={enabled}
                                    onClick={addTeam}
                                    id={String(team)}
                                    label={team}
                                    classes={'team' + team}
                                />
                            </div>
                        );
                    })}
                </div>
            ) : (
                <div className="envs-dropdown-select">{<div id="missing_envs">There are no teams to list</div>}</div>
            )}
        </ConfirmationDialog>
    );
};
