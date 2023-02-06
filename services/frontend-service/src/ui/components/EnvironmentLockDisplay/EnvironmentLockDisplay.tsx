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
