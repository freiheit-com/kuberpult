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

export const GitSyncStatusDescription: React.FC<{ status: number | undefined }> = (props) => {
    const { status } = props;
    if (status === undefined) {
        return <span className="rollout__description_unknown">? Unknown</span>;
    }
    switch (status) {
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCED:
            return <span className="rollout__description_successful">✓ Done</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED:
            return <span className="rollout__description_progressing">↻ In progress</span>;
        case GitSyncStatus.GIT_SYNC_STATUS_ERROR:
            return <span className="rollout__description_error">! Failed</span>;
    }
    return <span className="rollout__description_unknown">? Unknown</span>;
};
