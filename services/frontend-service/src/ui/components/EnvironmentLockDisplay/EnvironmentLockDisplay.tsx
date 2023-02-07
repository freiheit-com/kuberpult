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
import * as React from 'react';
import { addAction, useEnvironmentLock } from '../../utils/store';
import Tooltip from '@material-ui/core/Tooltip';
import { Locks } from '../../../images';
import { Button } from '../button';

export const EnvironmentLockDisplay: React.FC<{ env: string; lockId: string }> = (props) => {
    const { env, lockId } = props;
    const lock = useEnvironmentLock(lockId);
    const deleteLock = React.useCallback(() => {
        addAction({
            action: { $case: 'deleteEnvironmentLock', deleteEnvironmentLock: { environment: env, lockId: lockId } },
        });
    }, [env, lockId]);
    return (
        <div className="environment-lock-display">
            <Tooltip
                arrow
                title={'Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}>
                <div>
                    <Button
                        icon={<Locks className="environment-lock-icon" />}
                        onClick={deleteLock}
                        className={'button-lock'}
                    />
                </div>
            </Tooltip>
        </div>
    );
};
