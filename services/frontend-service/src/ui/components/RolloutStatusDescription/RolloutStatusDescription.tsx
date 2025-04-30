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

/*
 * div class="env-card-header"><div class="mdc-evolution-chip-release-dialog release-environment" role="row"><span class="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary" role="gridcell"><span class="mdc-evolution-chip__text-name"><span class="env-card-header-name">prod</span></span> <span class="mdc-evolution-chip__text-numbers"></span><div class="release-environment env-locks"></div></span></div><div class="env-card-locks"><div class="tooltip-container" id="tooltip" data-tooltip-place="bottom" data-tooltip-delay-hide="50" data-tooltip-id="kuberpult-tooltip" data-tooltip-html="<div class=&quot;mdc-tooltip__title_ release__details&quot;>ArgoCD has applied these changes, but some error has occurred.</div>"><span class="rollout__description_error">! Failed</span></div></div></div><div class="content-area"><div class="content-left"><div class="env-card-data" title="Shows the version that is currently deployed on prod. "><span><span class="release-version__display-version" title="cafe">2</span>the other commit message 2</span></div><div class="env-card-data"><div><a id="deployment-ci-link-prod-test1" class="deployment-ci-link" rel="noopener noreferrer" href="/www.somewebsite.com" target="_blank">Deployed by somebody </a>&nbsp;</div><div>&nbsp;</div></div><div class="env-card-data"><div>same version</div></div></div><div class="content-right"><div class="env-card-buttons"><div title="When doing manual deployments, it is usually best to also lock the app. If you omit the lock, an automatic release train or another person may deploy an unintended version."><div class="deploy-lock-buttons"><button class="mdc-button button-popup-lock env-card-lock-btn mdc-button--unelevated highlight" aria-label="Add Lock Only"><div class="mdc-button__ripple"></div><span class="mdc-button__label">Add Lock Only</span></button><button disabled="" class="mdc-button button-main env-card-deploy-btn mdc-button--unelevated" aria-label="Deploy and Lock"><div class="mdc-button__ripple"></div><span class="mdc-button__label">Deploy and Lock</span></button></div></div></div></div></div>
 * */
