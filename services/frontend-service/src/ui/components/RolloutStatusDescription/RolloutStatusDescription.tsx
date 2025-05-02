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
import { RolloutStatus } from '../../../api/api';
import React from 'react';
import { Tooltip } from '../tooltip/tooltip';
import { Argo, Git } from '../../../images';
import { GitSyncStatusDescription } from '../GitSyncStatusDescription/GitSyncStatusDescription';

const ROLLOUT_STATUS_UNKNOWN_DESCRIPTION =
    "ArgoCD hasn't reported any information about this application on this environment.";

const ROLLOUT_STATUS_SUCCESSFUL_DESCRIPTION = 'ArgoCD has successfully synced this application this environment.';
const ROLLOUT_STATUS_PROGRESSING_DESCRIPTION =
    'ArgoCD has picked up these changes for this application on this environment, but has not applied them yet. This process might take a while.';
const ROLLOUT_STATUS_PENDING_DESCRIPTION = 'ArgoCD has not yet picked up these changes.';

const ROLLOUT_STATUS_ERROR_DESCRIPTION = 'ArgoCD has applied these changes, but some error has occurred.';
const ROLLOUT_STATUS_UNHEALTHY_DESCRIPTION =
    'ArgoCD applied the changes successfully, but the application is unhealthy.';

export const RolloutStatusDescription: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;

    let span = <span className="rollout__description_unknown">? Unknown</span>;
    let tooltipContent = ROLLOUT_STATUS_UNKNOWN_DESCRIPTION;
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            span = <span className="rollout__description_successful">✓ Done</span>;
            tooltipContent = ROLLOUT_STATUS_SUCCESSFUL_DESCRIPTION;
            break;
        case RolloutStatus.ROLLOUT_STATUS_PROGRESSING:
            span = <span className="rollout__description_progressing">↻ In progress</span>;
            tooltipContent = ROLLOUT_STATUS_PROGRESSING_DESCRIPTION;
            break;
        case RolloutStatus.ROLLOUT_STATUS_PENDING:
            span = <span className="rollout__description_pending">⧖ Pending</span>;
            tooltipContent = ROLLOUT_STATUS_PENDING_DESCRIPTION;
            break;
        case RolloutStatus.ROLLOUT_STATUS_ERROR:
            span = <span className="rollout__description_error">! Failed</span>;
            tooltipContent = ROLLOUT_STATUS_ERROR_DESCRIPTION;
            break;
        case RolloutStatus.ROLLOUT_STATUS_UNHEALTHY:
            span = <span className="rollout__description_unhealthy">⚠ Unhealthy</span>;
            tooltipContent = ROLLOUT_STATUS_UNHEALTHY_DESCRIPTION;
            break;
    }
    return (
        <Tooltip tooltipContent={<div className="mdc-tooltip__title_ release__details">{tooltipContent}</div>}>
            {span}
        </Tooltip>
    );
};

export const AAEnvironmentRolloutDescription: React.FC<{ statuses: [string, RolloutStatus | undefined][] }> = (
    props
) => {
    const { statuses } = props;
    const span = <span className="rollout__description_unknown">? Unknown</span>;
    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {statuses.length > 0 && (
                <table className="release__environment_status">
                    <thead>
                        <tr>
                            <th className={'tooltip-text'}>Environment group</th>
                            {
                                <th className="release-card__statusth tooltip-text">
                                    Rollout Status <Argo className="status-logo" />
                                </th>
                            }
                        </tr>
                    </thead>
                    <tbody>
                        {statuses.map((currentStatus) => (
                            <tr key={currentStatus[0]}>
                                <td className={'tooltip-text'}>{currentStatus[0]}</td>
                                <td>
                                    <RolloutStatusDescription
                                        status={
                                            currentStatus[1] ? currentStatus[1] : RolloutStatus.ROLLOUT_STATUS_UNKNOWN
                                        }
                                    />
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}
        </div>
    );
    return (
        <Tooltip tooltipContent={<div className="mdc-tooltip__title_ release__details">{tooltipContents}</div>}>
            {span}
        </Tooltip>
    );
};
