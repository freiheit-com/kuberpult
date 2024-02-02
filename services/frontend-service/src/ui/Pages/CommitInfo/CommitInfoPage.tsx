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

Copyright 2023 freiheit.com*/

import {
    getCommitInfo,
    // showSnackbarError,
    useCommitInfo,
    useGlobalLoadingState,
    CommitInfoState,
} from '../../utils/store';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { Spinner } from '../../components/Spinner/Spinner';
import { useParams } from 'react-router-dom';
import React from 'react';
import { CommitInfo } from '../../components/CommitInfo/CommitInfo';

export const CommitInfoPage: React.FC = () => {
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    const { commit: commitHash } = useParams();

    React.useEffect(() => {
        if (commitHash !== undefined) {
            getCommitInfo(commitHash);
        }
    }, [commitHash]);

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
        case CommitInfoState.READY:
            return <CommitInfo commitHash={commitHash} commitInfo={commitInfo.response} />;
    }
};
