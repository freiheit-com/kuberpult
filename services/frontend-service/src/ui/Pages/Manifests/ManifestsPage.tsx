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
    useGlobalLoadingState,
    getManifest,
    useManifestInfo,
    ManifestRequestState,
    useEnvironmentNames,
} from '../../utils/store';
import { Spinner } from '../../components/Spinner/Spinner';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import React from 'react';
import { useSearchParams } from 'react-router-dom';
import { Manifest } from '../../components/Manifests/Manifests';

export const ManifestsPage: React.FC = () => {
    const { authHeader } = useAzureAuthSub((auth) => auth);
    const [urlParameters] = useSearchParams();
    const envs = useEnvironmentNames();
    const applicationParam = urlParameters.get('app');
    const releaseParam = urlParameters.get('release');
    const application = applicationParam ? applicationParam : '';
    const releaseNumber = releaseParam ? releaseParam : '';
    const manifestResponse = useManifestInfo((res) => res);

    React.useEffect(() => {
        getManifest(application, releaseNumber, authHeader);
    }, [application, releaseNumber, authHeader]);

    const element = useGlobalLoadingState();
    if (element) {
        return element;
    }

    switch (manifestResponse.manifestInfoReady) {
        case ManifestRequestState.LOADING:
            return <Spinner message="Loading manifest..." />;
        case ManifestRequestState.ERROR:
            return (
                <div>
                    <main className="main-content manifests-page">
                        Something went wrong fetching data from Kuberpult.
                    </main>
                </div>
            );
        case ManifestRequestState.NOTFOUND:
            return (
                <div>
                    <main className="main-content manifests-page">
                        <h1>
                            Kuberpult could not find the manifests for release {releaseNumber} of {application}.
                        </h1>
                    </main>
                </div>
            );
        case ManifestRequestState.READY:
            return (
                <div>
                    <main className="manifests-page main-content">
                        <h1>
                            Manifests for release {releaseNumber} of '{applicationParam}'.
                        </h1>
                        {manifestResponse.response ? (
                            envs.map((currentEnv) =>
                                manifestResponse.response?.manifests[currentEnv] ? (
                                    <Manifest
                                        key={currentEnv}
                                        Manifest={manifestResponse.response?.manifests[currentEnv].content}
                                        EnvironmentName={currentEnv}
                                    />
                                ) : (
                                    <div key={currentEnv}>
                                        <h2>No Manifest found for environment: {currentEnv}.</h2>
                                        <hr />
                                    </div>
                                )
                            )
                        ) : (
                            <div>'No manifests were found'</div>
                        )}
                    </main>
                </div>
            );
    }
};
