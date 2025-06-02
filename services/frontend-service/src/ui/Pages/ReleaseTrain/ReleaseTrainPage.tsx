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

import { useParams } from 'react-router-dom';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import {
    ReleaseTrainPrognosisState,
    getReleaseTrainPrognosis,
    useGlobalLoadingState,
    useReleaseTrainPrognosis,
} from '../../utils/store';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import React from 'react';
import { Spinner } from '../../components/Spinner/Spinner';
import { ReleaseTrainPrognosis } from '../../components/ReleaseTrainPrognosis/ReleaseTrainPrognosis';

export const ReleaseTrainPage: React.FC = () => {
    const { targetEnv: envName } = useParams();
    const { authHeader } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        if (envName !== undefined) {
            getReleaseTrainPrognosis(envName, authHeader);
        }
    }, [envName, authHeader]);

    const releaseTrainPrognosis = useReleaseTrainPrognosis((res) => res);

    const element = useGlobalLoadingState();
    if (element) {
        return element;
    }

    if (envName === undefined) {
        return (
            <div>
                <TopAppBar
                    showAppFilter={false}
                    showTeamFilter={false}
                    showWarningFilter={false}
                    showGitSyncStatus={false}
                />
                <main className="main-content">Environment name not provided</main>
            </div>
        );
    }
    let page = <main className="main-content">Backend error</main>;
    switch (releaseTrainPrognosis.releaseTrainPrognosisReady) {
        case ReleaseTrainPrognosisState.LOADING:
            return <Spinner message="Loading release train prognosis" />;
        case ReleaseTrainPrognosisState.ERROR:
            page = <main className="main-content">Backend error</main>;
            break;
        case ReleaseTrainPrognosisState.NOTFOUND:
            page = (
                <main className="main-content">
                    The provided environment name {envName} was not found in the manifest repository.
                </main>
            );
            break;
        case ReleaseTrainPrognosisState.READY:
            page = <ReleaseTrainPrognosis releaseTrainPrognosis={releaseTrainPrognosis.response} />;
            break;
    }
    return (
        <div className="release-train-prognosis">
            <TopAppBar
                showAppFilter={false}
                showTeamFilter={false}
                showWarningFilter={false}
                showGitSyncStatus={false}
            />
            {page}
        </div>
    );
};
