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

import { getFailedEsls, useFailedEsls, useGlobalLoadingState, FailedEslsState } from '../../utils/store';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { Spinner } from '../../components/Spinner/Spinner';
import React from 'react';
import { EslWarnings } from '../../components/EslWarnings/EslWarnings';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';

export const EslWarningsPage: React.FC = () => {
    const { authHeader } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        getFailedEsls(authHeader);
    }, [authHeader]);

    const failedEsls = useFailedEsls((res) => res);

    const element = useGlobalLoadingState();
    if (element) {
        return element;
    }

    switch (failedEsls.failedEslsReady) {
        case FailedEslsState.LOADING:
            return <Spinner message="Loading Failed Esls info" />;
        case FailedEslsState.ERROR:
            return (
                <div>
                    <TopAppBar
                        showAppFilter={false}
                        showTeamFilter={false}
                        showWarningFilter={false}
                        showGitSyncStatus={false}
                    />
                    <main className="main-content esl-warnings-page">Backend error</main>
                </div>
            );
        case FailedEslsState.NOTFOUND:
            return (
                <div>
                    <TopAppBar
                        showAppFilter={false}
                        showTeamFilter={false}
                        showWarningFilter={false}
                        showGitSyncStatus={false}
                    />
                    <main className="main-content esl-warnings-page">
                        <p>All events were processed successfully</p>
                    </main>
                </div>
            );
        case FailedEslsState.READY:
            return (
                <div>
                    <TopAppBar
                        showAppFilter={false}
                        showTeamFilter={false}
                        showWarningFilter={false}
                        showGitSyncStatus={false}
                    />
                    <EslWarnings failedEsls={failedEsls.response} />;
                </div>
            );
    }
};
