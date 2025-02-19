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

import { useGitSyncStatus } from '../../utils/store';
import { GitSyncStatus } from '../../../api/api';
import { SmallSpinner } from '../Spinner/Spinner';
import { Git } from '../../../images';

export type GeneralGitSyncStatusProps = {
    enabled: boolean;
};

export const GeneralGitSyncStatus: React.FC<GeneralGitSyncStatusProps> = (props) => {
    const gitSyncStatus = useGitSyncStatus((m) => m);
    let label;
    switch (gitSyncStatus.getHighestPriorityGitStatus()) {
        case GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED:
            label = <UnsyncedGeneralSynStatus></UnsyncedGeneralSynStatus>;
            break;
        case GitSyncStatus.GIT_SYNC_STATUS_SYNCED:
            label = <SyncedGeneralSynStatus></SyncedGeneralSynStatus>;
            break;
        case GitSyncStatus.GIT_SYNC_STATUS_ERROR:
            label = <ErrorGeneralSynStatus></ErrorGeneralSynStatus>;
            break;
        default:
            label = <UnknownGeneralSynStatus></UnknownGeneralSynStatus>;
    }
    return <div className={'mdc-top-app-bar__section top-app-bar--narrow-filter'}> {props.enabled ? label : ''}</div>;
};

export const SyncedGeneralSynStatus: React.FC = () => (
    <div className="top-app-bar-search-field general-status__synced">
        <Git className="logo" />
        <span className="welcome-message">✓</span>
    </div>
);
export const UnknownGeneralSynStatus: React.FC = () => (
    <div className="top-app-bar-search-field general-status__unknown">
        <Git className="logo" />
        <span className="welcome-message">?</span>
    </div>
);
export const UnsyncedGeneralSynStatus: React.FC = () => (
    <div className="top-app-bar-search-field general-status__unsynced">
        <Git className="logo" />
        <SmallSpinner appName={'general-sync-status'} size={20}></SmallSpinner>
    </div>
);
export const ErrorGeneralSynStatus: React.FC = () => (
    <div className="top-app-bar-search-field general-status__error">
        <Git className="logo" />
        <span className="welcome-message">!</span>
    </div>
);
