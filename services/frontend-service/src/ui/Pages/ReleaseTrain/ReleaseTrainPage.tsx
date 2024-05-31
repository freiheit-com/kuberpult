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
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import {
    ReleaseTrainPrognosisState,
    getReleaseTrainPrognosis,
    useGlobalLoadingState,
    useReleaseTrainPrognosis,
} from '../../utils/store';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import React from 'react';
import { Spinner } from '../../components/Spinner/Spinner';
import { ReleaseTrainPrognosis } from '../../components/ReleaseTrain/ReleaseTrain';

export const ReleaseTrainPage: React.FC = () => {
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    const { targetEnv: envName } = useParams();
    const { authHeader } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        if (envName !== undefined) {
            getReleaseTrainPrognosis(envName, authHeader);
        }
    }, [envName, authHeader]);

    const releaseTrainPrognosis = useReleaseTrainPrognosis((res) => res);

    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }

    if (envName === undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content">Environment name not provided</main>
            </div>
        );
    }

    switch (releaseTrainPrognosis.releaseTrainPrognosisReady) {
        case ReleaseTrainPrognosisState.LOADING:
            return <Spinner message="Loading release train prognosis" />;
        case ReleaseTrainPrognosisState.ERROR:
            return (
                <div>
                    <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                    <main className="main-content">Backend error</main>
                </div>
            );
        case ReleaseTrainPrognosisState.NOTFOUND:
            return (
                <div>
                    <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                    <main className="main-content">
                        The provided environment name {envName} was not found in the manifest repository.
                    </main>
                </div>
            );
        case ReleaseTrainPrognosisState.READY:
            return <ReleaseTrainPrognosis releaseTrainPrognosis={releaseTrainPrognosis.response} />;
    }
};
