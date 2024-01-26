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

import { getCommitInfo, useCommitInfo, useGlobalLoadingState } from '../../utils/store';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import { useParams } from 'react-router-dom';
// import { GetCommitInfoResponse } from '../../../api/api';

// type Commit = {
//     hash: string;
//     message: string;
//     apps: string[];
// };

export const CommitPage: React.FC = () => {
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    const { commit } = useParams();
    const commitInfo = useCommitInfo((res) => res);
    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }
    // this dummy data will be replaced in SRX-2ZMPC4
    // const commitData: Commit = {
    //     hash: commit || 'unknown',
    //     message: 'UX: make submit button bigger\n\nanother message\nMore text',
    //     apps: ['echo', 'customer-data', 'ui', 'bff'],
    // };

    if (commit !== undefined) getCommitInfo(commit);
    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content commit-page">
                <h1>This page is still in beta</h1>
                <br />
                <h1> Commit {commitInfo.response.commitMessage} </h1>
                <table border={1}>
                    <thead>
                        <tr>
                            <th>Commit Hash:</th>
                            <th>Commit Message:</th>
                            <th>Touched apps: </th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>{commit}</td>
                            <td>
                                <div className={'commit-page-message'}>
                                    {commitInfo.response.commitMessage.split('\n').map((msg, index) => (
                                        <div key={index}>{msg} &nbsp;</div>
                                    ))}
                                </div>
                            </td>
                            <td>{commitInfo.response.touchedApps.join(', ')}</td>
                        </tr>
                    </tbody>
                </table>
            </main>
        </div>
    );
};
