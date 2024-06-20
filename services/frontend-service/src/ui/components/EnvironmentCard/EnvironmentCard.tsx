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
import { addAction, getPriorityClassName, useFilteredEnvironmentLockIDs, useTeamNames } from '../../utils/store';
import { Button } from '../button';
import { Locks } from '../../../images';
import * as React from 'react';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';
import { Environment, EnvironmentGroup } from '../../../api/api';
import classNames from 'classnames';
import { ProductVersionLink, ReleaseTrainLink, setOpenEnvironmentConfigDialog } from '../../utils/Links';
import { useSearchParams } from 'react-router-dom';
import { useCallback, useState } from 'react';
import { TeamSelectionDialog } from '../SelectionDialog/SelectionDialogs';

export const EnvironmentCard: React.FC<{ environment: Environment; group: EnvironmentGroup | undefined }> = (props) => {
    const { environment, group } = props;
    const [params, setParams] = useSearchParams();
    const locks = useFilteredEnvironmentLockIDs(environment.name);
    const teams = useTeamNames();

    const priorityClassName = group !== undefined ? getPriorityClassName(group) : getPriorityClassName(environment);
    const onShowConfigClick = useCallback((): void => {
        setOpenEnvironmentConfigDialog(params, environment.name);
        setParams(params);
    }, [environment.name, params, setParams]);
    const [showTeamSelectionDialog, setShowTeamSelectionDialog] = useState(false);
    const addLock = React.useCallback(() => {
        addAction({
            action: {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: { environment: environment.name, lockId: '', message: '' },
            },
        });
    }, [environment.name]);

    const popupSelectTeams = React.useCallback(() => {
        setShowTeamSelectionDialog(true);
    }, [setShowTeamSelectionDialog]);

    const handleCloseTeamSelectionDialog = useCallback(() => {
        setShowTeamSelectionDialog(false);
    }, []);
    const confirmTeamLockCreate = useCallback(
        (selectedTeams: string[]) => {
            selectedTeams.forEach((team) => {
                addAction({
                    action: {
                        $case: 'createEnvironmentTeamLock',
                        createEnvironmentTeamLock: {
                            team: team,
                            environment: environment.name,
                            lockId: '',
                            message: '',
                        },
                    },
                });
            });
            setShowTeamSelectionDialog(false);
        },
        [environment.name]
    );
    const dialog = (
        <TeamSelectionDialog
            teams={teams}
            onSubmit={confirmTeamLockCreate}
            onCancel={handleCloseTeamSelectionDialog}
            open={showTeamSelectionDialog}
            multiselect={true}></TeamSelectionDialog>
    );

    return (
        <div className="environment-lane">
            {dialog}
            <div className={classNames('environment-lane__header', priorityClassName)}>
                <div className="environment__name" title={'Name of the environment'}>
                    {environment.name}
                </div>
            </div>
            <div className="environment-lane__body">
                {locks.length !== 0 && (
                    <div className="environment__locks">
                        {locks.map((lock) => (
                            <EnvironmentLockDisplay
                                env={environment.name}
                                lockId={lock}
                                key={lock}
                                bigLockIcon={true}
                            />
                        ))}
                    </div>
                )}
                <div className="environment__actions">
                    <Button
                        className="environment-action service-action--prepare-undeploy test-lock-env"
                        label={'Add Environment Lock in ' + environment.name}
                        icon={<Locks />}
                        onClick={addLock}
                        highlightEffect={false}
                    />
                    <Button
                        className="environment-action service-action--prepare-undeploy test-lock-env"
                        label={'Add Team Lock in ' + environment.name}
                        icon={<Locks />}
                        onClick={popupSelectTeams}
                        highlightEffect={false}
                    />
                    <Button
                        className="environment-action service-action--show-config"
                        label={'Show Configuration of environment ' + environment.name}
                        onClick={onShowConfigClick}
                        highlightEffect={false}
                    />
                    <div>
                        <div className="environment-link">
                            <ReleaseTrainLink env={environment.name} />
                        </div>
                        <div className="environment-link">
                            <ProductVersionLink env={environment.name} groupName={group?.environmentGroupName ?? ''} />
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export const EnvironmentGroupCard: React.FC<{ environmentGroup: EnvironmentGroup }> = (props) => {
    const { environmentGroup } = props;
    // all envs in the same group have the same priority
    const priorityClassName = getPriorityClassName(environmentGroup);
    const addLock = React.useCallback(() => {
        environmentGroup.environments.forEach((environment) => {
            addAction({
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: { environment: environment.name, lockId: '', message: '' },
                },
            });
        });
    }, [environmentGroup]);
    return (
        <div className="environment-group-lane">
            <div className="environment-group-lane__header-wrapper">
                <div className={classNames('environment-group-lane__header', priorityClassName)}>
                    <div className="environment-group__name" title={'Name of the environment group'}>
                        {environmentGroup.environmentGroupName}
                    </div>
                </div>
                <div className="environment__actions">
                    <Button
                        className="environment-action service-action--prepare-undeploy test-lock-group"
                        label={'Add Lock for each environment in ' + environmentGroup.environmentGroupName}
                        icon={<Locks />}
                        onClick={addLock}
                        highlightEffect={false}
                    />
                </div>
            </div>
            <div className="environment-group-lane__body">
                {environmentGroup.environments.map((env) => (
                    <EnvironmentCard environment={env} key={env.name} group={environmentGroup} />
                ))}
            </div>
            {/*I am just here so that we can avoid margin collapsing */}
            <div className={'environment-group-lane__footer'} />
        </div>
    );
};
