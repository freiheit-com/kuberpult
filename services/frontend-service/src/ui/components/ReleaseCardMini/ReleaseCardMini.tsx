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
import classNames from 'classnames';
import React from 'react';
import { updateReleaseDialog, useCurrentlyDeployedAt, useOverview, useRelease } from '../../utils/store';
import { Chip } from '../chip';
import { calculateEnvironmentPriorities, EnvPrioMap, sortEnvironmentsByUpstream } from '../ReleaseDialog/ReleaseDialog';
import { Environment } from '../../../api/api';

export type ReleaseCardMiniProps = {
    className?: string;
    version: number;
    app: string;
};

const getDays = (date: Date) => {
    const current = new Date(Date.now());
    const diff = current.getTime() - date.getTime();

    return Math.round(diff / (1000 * 3600 * 24));
};

export const ReleaseCardMini: React.FC<ReleaseCardMiniProps> = (props) => {
    const { className, app, version } = props;
    const { createdAt, sourceMessage, sourceAuthor } = useRelease(app, version);
    const environmentsForApp = useCurrentlyDeployedAt(app, version);
    const clickHanlder = React.useCallback(() => {
        updateReleaseDialog(app, version);
    }, [app, version]);
    let msg = sourceAuthor;
    if (createdAt !== undefined) {
        const days = getDays(createdAt);
        msg += ' commited ';

        if (days === 0) {
            msg += 'at ';
        } else {
            msg += days + ' days ago at ';
        }
        msg += `${createdAt.getHours()}:${createdAt.getMinutes()}:${createdAt.getSeconds()}`;
    }
    const allEnvs: Environment[] = useOverview((x) => Object.values(x.environments));
    const sortedEnvs: Environment[] = sortEnvironmentsByUpstream(allEnvs);
    const envPrioMap: EnvPrioMap = calculateEnvironmentPriorities(allEnvs);
    const envsForAppSorted = sortedEnvs.filter((env: Environment) => environmentsForApp.includes(env));

    return (
        <div className={classNames('release-card-mini', className)} onClick={clickHanlder}>
            <div className={classNames('release__details-mini', className)}>
                <div className="release__details-header">{sourceMessage}</div>
                <div className="release__details-msg">{msg}</div>
            </div>
            <div className="release__environments-mini">
                {envsForAppSorted.map((env) => (
                    <Chip
                        className={classNames('release-environment', className)}
                        label={env.name}
                        key={env.name}
                        priority={envPrioMap[env.name]}
                    />
                ))}
            </div>
        </div>
    );
};
