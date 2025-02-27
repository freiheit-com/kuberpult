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

const ROLLOUT_STATUS_UNKNOWN_DESCRIPTION =
    "ArgoCD hasn't reported any information about this application on this environment.";

const ROLLOUT_STATUS_SUCCESFUL_DESCRIPTION = 'ArgoCD has successfully synced this application this environment.';
const ROLLOUT_STATUS_PROGRESSING_DESCRIPTION =
    'ArgoCD has picked up these changes for this application on this environment, but has not applied them yet. This process might take a while.';
const ROLLOUT_STATUS_PENDING_DESCRIPTION = 'ArgoCD has not yet picked up these changes.';
const ROLLOUT_STATUS_ERROR_DESCRIPTION = 'ArgoCD has applied these changes, but some error has occurred';
const ROLLOUT_STATUS_UNHEALTHY_DESCRIPTION =
    'ArgoCD applied the changes successfully, but the application is unhealthy';

export const RolloutStatusDescription: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;

    let span = <span className="rollout__description_unknown">? Unknown</span>;
    let tooltipContent = ROLLOUT_STATUS_UNKNOWN_DESCRIPTION;
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            span = <span className="rollout__description_successful">✓ Done</span>;
            tooltipContent = ROLLOUT_STATUS_SUCCESFUL_DESCRIPTION;
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
