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

import React from 'react';
import { GitSyncStatus } from '../../../api/api';
import { Tooltip } from '../tooltip/tooltip';

const GIT_SYNC_STATUS_SYNCED_DESCRIPTION =
    'All changes to this app and environment have been processed and pushed to the manifest repository successfully.';

const GIT_SYNC_STATUS_UNSYNCED_DESCRIPTION =
    'There are some changes to this app and environment that have been processed by Kuberpult, but are yet to be committed to the manifest repository.';
const GIT_SYNC_STATUS_FAILED_DESCRIPTION =
    "There are some changes to this app and environment that have been processed by Kuberpult, but it wasn't possible to push the changes to the manifest repository. This requires manual intervention.";
const GIT_SYNC_STATUS_UNKOWN_DESCRIPTION = 'Kuberpult could not find the git sync status for this app and environment.';

export const GitSyncStatusDescription: React.FC<{ status: number | undefined }> = (props) => {
    const { status } = props;
    let span = <span className="rollout__description_unknown">? Unknown</span>;
    let tooltipContent = GIT_SYNC_STATUS_UNKOWN_DESCRIPTION;
    if (status === undefined) {
        return (
            <Tooltip tooltipContent={<div className="mdc-tooltip__title_ release__details">{tooltipContent}</div>}>
                {span}
            </Tooltip>
        );
    }
    switch (status) {
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCED:
            span = <span className="rollout__description_successful">✓ Done</span>;
            tooltipContent = GIT_SYNC_STATUS_SYNCED_DESCRIPTION;
            break;
        case GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED:
            span = <span className="rollout__description_progressing">↻ In progress</span>;
            tooltipContent = GIT_SYNC_STATUS_UNSYNCED_DESCRIPTION;
            break;
        case GitSyncStatus.GIT_SYNC_STATUS_ERROR:
            span = <span className="rollout__description_error">! Failed</span>;
            tooltipContent = GIT_SYNC_STATUS_FAILED_DESCRIPTION;
            break;
    }
    return (
        <Tooltip tooltipContent={<div className="mdc-tooltip__title_ release__details">{tooltipContent}</div>}>
            {span}
        </Tooltip>
    );
};
