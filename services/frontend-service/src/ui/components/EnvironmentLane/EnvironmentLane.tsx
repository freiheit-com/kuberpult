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
import { useFilteredEnvironmentLocks } from '../../utils/store';
import { Button } from '../button';
import { Delete } from '../../../images';
import * as React from 'react';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';

export const EnvironmentLane: React.FC<{ environment: string }> = (props) => {
    const { environment } = props;
    const locks = useFilteredEnvironmentLocks(environment);
    return (
        <div className="environment-lane">
            <div className="environment-lane__header">
                <div className="environment__name">{environment}</div>
            </div>
            {locks.length !== 0 && (
                <div className="environment__locks">
                    {locks.map((lock) => (
                        <EnvironmentLockDisplay lockId={lock} key={lock} />
                    ))}
                </div>
            )}
            <div className="environment__actions">
                <Button
                    className="environment-action service-action--prepare-undeploy"
                    label={'Add Lock'}
                    icon={<Delete />}
                />
            </div>
        </div>
    );
};
