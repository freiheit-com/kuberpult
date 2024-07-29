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

import {
    getCommitInfo,
    useCommitInfo,
    useGlobalLoadingState,
    CommitInfoState,
    updateCommitInfo,
} from '../../utils/store';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { Spinner } from '../../components/Spinner/Spinner';
import { useParams } from 'react-router-dom';
import React, { useCallback } from 'react';
import { CommitInfo } from '../../components/CommitInfo/CommitInfo';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';

export const CommitInfoPage: React.FC = () => {
    const { commit: commitHash } = useParams();
    const { authHeader } = useAzureAuthSub((auth) => auth);
    const [eventLimit, setLimit] = React.useState(1);
    const increment: number = 10;

    React.useEffect(() => {
        if (commitHash !== undefined) {
            getCommitInfo(commitHash, eventLimit, authHeader);
        }
    }, [commitHash, authHeader, eventLimit]);

    const triggerLoadMore = useCallback(() => {
        setLimit(eventLimit + increment);
        updateCommitInfo.set({ commitInfoReady: CommitInfoState.LOADING });
    }, [eventLimit]);

    const commitInfo = useCommitInfo((res) => res);

    const element = useGlobalLoadingState();
    if (element) {
        return element;
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
                    {/*<TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />*/}
                    <main className="main-content commit-page">
                        The provided commit ID was not found in the manifest repository or database. This is because
                        either the commit "{commitHash}" is incorrect, is not tracked by Kuberpult yet, or it refers to
                        an old commit whose release has been cleaned up by now.
                    </main>
                </div>
            );
        case CommitInfoState.READY:
            return (
                <CommitInfo
                    commitInfo={commitInfo.response}
                    triggerLoadMore={triggerLoadMore}
                    eventLimit={eventLimit}
                />
            );
    }
};
