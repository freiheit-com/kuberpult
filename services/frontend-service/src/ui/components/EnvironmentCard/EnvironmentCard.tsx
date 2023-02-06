import { addAction, useFilteredEnvironmentLockIDs } from '../../utils/store';
import { Button } from '../button';
import { Locks } from '../../../images';
import * as React from 'react';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';

export const EnvironmentCard: React.FC<{ environment: string }> = (props) => {
    const { environment } = props;
    const locks = useFilteredEnvironmentLockIDs(environment);

    const addLock = React.useCallback(() => {
        const randBase36 = () => Math.random().toString(36).substring(7);
        const randomLockId = () => 'ui-v2-' + randBase36();
        addAction({
            action: {
                $case: 'createEnvironmentLock',
                createEnvironmentLock: { environment: environment, lockId: randomLockId(), message: '' },
            },
        });
    }, [environment]);
    return (
        <div className="environment-lane">
            <div className="environment-lane__header">
                <div className="environment__name">{environment}</div>
            </div>
            {locks.length !== 0 && (
                <div className="environment__locks">
                    {locks.map((lock) => (
                        <EnvironmentLockDisplay env={environment} lockId={lock} key={lock} />
                    ))}
                </div>
            )}
            <div className="environment__actions">
                <Button
                    className="environment-action service-action--prepare-undeploy"
                    label={'Add Lock'}
                    icon={<Locks />}
                    onClick={addLock}
                />
            </div>
        </div>
    );
};
