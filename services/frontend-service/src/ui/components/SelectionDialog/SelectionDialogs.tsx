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
import { showSnackbarError } from '../../utils/store';
import { GenericSelectionDialog } from './GenericSelectionDialog';

export type EnvSelectionDialogProps = {
    environments: string[];
    onSubmit: (selectedEnvs: string[]) => void;
    onCancel: () => void;
    open: boolean;
    envSelectionDialog: boolean; // false if release train dialog
};

export const EnvSelectionDialog: React.FC<EnvSelectionDialogProps> = (props) => {
    const [selectedEnvs, setSelectedEnvs] = useState<string[]>([]);

    const onConfirm = React.useCallback(() => {
        if (selectedEnvs.length < 1) {
            showSnackbarError('There needs to be at least one environment selected to perform this action');
            return;
        }
        props.onSubmit(selectedEnvs);
        setSelectedEnvs([]);
    }, [props, selectedEnvs]);

    const onCancel = React.useCallback(() => {
        props.onCancel();
        setSelectedEnvs([]);
    }, [props]);

    const headerLabel = props.envSelectionDialog
        ? 'Select all environments to be removed:'
        : 'Select which environments to run release train to:';
    const confirmLabel = props.envSelectionDialog ? 'Remove app from environments' : 'Release Train';
    const onEmptyLabel = props.envSelectionDialog
        ? 'There are no environments to list'
        : 'There are no available environments to run a release train to based on the current environment/environmentGroup';

    return (
        <GenericSelectionDialog
            selectables={props.environments}
            open={props.open}
            onSubmit={onConfirm}
            onCancel={onCancel}
            multiSelect={props.envSelectionDialog}
            confirmLabel={confirmLabel}
            headerLabel={headerLabel}
            onEmptyLabel={onEmptyLabel}
            selectedItems={selectedEnvs}
            setSelectedItems={setSelectedEnvs}
        />
    );
};

export type TeamSelectionDialogProps = {
    teams: string[];
    onSubmit: (selectedTeams: string[]) => void;
    onCancel: () => void;
    open: boolean;
    multiselect: boolean;
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
    return (
        <GenericSelectionDialog
            selectables={props.teams}
            open={props.open}
            onSubmit={onConfirm}
            onCancel={onCancel}
            multiSelect={props.multiselect}
            confirmLabel={'Select Teams'}
            headerLabel={'Select teams for team lock:'}
            onEmptyLabel={'No teams to show.'}
            selectedItems={selectedTeams}
            setSelectedItems={setSelectedTeams}
        />
    );
};
