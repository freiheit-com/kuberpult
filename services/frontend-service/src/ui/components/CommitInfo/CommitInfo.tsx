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

import { TopAppBar } from '../../components/TopAppBar/TopAppBar';
import React from 'react';
import { GetCommitInfoResponse } from '../../../api/api';

type CommitInfoProps = {
    commitHash: string;
    commitInfo: GetCommitInfoResponse | undefined;
};

export const CommitInfo: React.FC<CommitInfoProps> = (props) => {
    const commitHash = props.commitHash;
    const commitInfo = props.commitInfo;
    if (commitInfo !== undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">
                    <h1>This page is still in beta</h1>
                    <br />
                    <h1> Commit {commitInfo.commitMessage.split('\n')[0]} </h1>
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
                                <td>{commitHash}</td>
                                <td>
                                    <div className={'commit-page-message'}>
                                        {commitInfo.commitMessage.split('\n').map((msg, index) => (
                                            <div key={index}>{msg} &nbsp;</div>
                                        ))}
                                    </div>
                                </td>
                                <td>{commitInfo.touchedApps.join(', ')}</td>
                            </tr>
                        </tbody>
                    </table>
                </main>
            </div>
        );
    } else {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">Backend returned empty response</main>
            </div>
        );
    }
};
