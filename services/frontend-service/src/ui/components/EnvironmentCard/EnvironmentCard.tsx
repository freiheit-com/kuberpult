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

Copyright 2023 freiheit.com*/
import { addAction, useFilteredEnvironmentLockIDs, useLockId } from '../../utils/store';
import { Button } from '../button';
import { Locks } from '../../../images';
import * as React from 'react';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';
import { EnvironmentGroup } from '../../../api/api';

export const EnvironmentCard: React.FC<{ environment: string }> = (props) => {
    const { environment } = props;
    const locks = useFilteredEnvironmentLockIDs(environment);

    const lockId = useLockId();
    const addLock = React.useCallback(() => {
        addAction({
            action: {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: { environment: environment, lockId: lockId, message: '' },
            },
        });
    }, [environment, lockId]);
    return (
        <div className="environment-lane">
            <div className="environment-lane__header">
                <div className="environment__name" title={'Name of the environment'}>
                    {environment}
                </div>
            </div>
            <div className="environment-lane__body">
                {locks.length !== 0 && (
                    <div className="environment__locks">
                        {locks.map((lock) => (
                            <EnvironmentLockDisplay env={environment} lockId={lock} key={lock} />
                        ))}
                    </div>
                )}
                <div className="environment__actions">
                    <Button
                        className="environment-action service-action--prepare-undeploy test-lock-env"
                        label={'Add Environment Lock in ' + environment}
                        icon={<Locks />}
                        onClick={addLock}
                    />
                </div>
            </div>
        </div>
    );
};

export const EnvironmentGroupCard: React.FC<{ environmentGroup: EnvironmentGroup }> = (props) => {
    const { environmentGroup } = props;
    const lockId = useLockId();
    const addLock = React.useCallback(() => {
        environmentGroup.environments.forEach((environment) => {
            addAction({
                action: {
                    $case: 'createEnvironmentLock',
                    createEnvironmentLock: { environment: environment.name, lockId: lockId, message: '' },
                },
            });
        });
    }, [environmentGroup, lockId]);
    return (
        <div className="environment-group-lane">
            <div className="environment-group-lane__header-wrapper">
                <div className="environment-group-lane__header">
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
                    />
                </div>
            </div>
            <div className="environment-group-lane__body">
                {environmentGroup.environments.map((env) => (
                    <EnvironmentCard environment={env.name} key={env.name} />
                ))}
            </div>
            {/*I am just here so that we can avoid margin collapsing */}
            <div className={'environment-group-lane__footer'} />
        </div>
    );
};
