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
import * as React from 'react';
import { useEnvironmentLock } from '../../utils/store';
import Tooltip from '@material-ui/core/Tooltip';
import { Locks } from '../../../images';

export const EnvironmentLockDisplay: React.FC<{ lockId: string }> = (props) => {
    const { lockId } = props;
    const lock = useEnvironmentLock(lockId);

    return (
        <div className="environment-lock-display">
            <Tooltip
                arrow
                title={'Lock Message: "' + lock.message + '" | ID: "' + lock.lockId + '"  | Click to unlock. '}>
                <div role="button" onClick={test}>
                    <Locks className="environment-lock-icon" />
                </div>
            </Tooltip>
        </div>
    );
};
