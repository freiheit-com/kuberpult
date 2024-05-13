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

import { getCommitInfo, useCommitInfo, useGlobalLoadingState, CommitInfoState } from '../../utils/store';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { Spinner } from '../../components/Spinner/Spinner';
import { useParams } from 'react-router-dom';
import React from 'react';
import { CommitInfo } from '../../components/CommitInfo/CommitInfo';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';

export const CommitInfoPage: React.FC = () => {
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    const { commit: commitHash } = useParams();
    const { authHeader } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        if (commitHash !== undefined) {
            getCommitInfo(commitHash, authHeader);
        }
    }, [commitHash, authHeader]);

    const commitInfo = useCommitInfo((res) => res);

    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }

    if (commitHash === undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">commit ID not provided</main>
            </div>
        );
    }
    switch (commitInfo.commitInfoReady) {
        case CommitInfoState.LOADING:
            return <Spinner message="Loading commit info" />;
        case CommitInfoState.ERROR:
            return (
                <div>
                    <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                    <main className="main-content commit-page">Backend error</main>
                </div>
            );
        case CommitInfoState.NOTFOUND:
            return (
                <div>
                    <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                    <main className="main-content commit-page">
                        The provided commit ID was not found in the manifest repository. This is because either the
                        commit "{commitHash}" is incorrect, is not tracked by Kuberpult yet, or it refers to an old
                        commit whose release has been cleaned up by now.
                    </main>
                </div>
            );
        case CommitInfoState.READY:
            return <CommitInfo commitInfo={commitInfo.response} />;
    }
};
