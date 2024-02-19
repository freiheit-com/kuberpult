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

import { TopAppBar } from '../TopAppBar/TopAppBar';
import React from 'react';
import { GetCommitInfoResponse, Event } from '../../../api/api';

type CommitInfoProps = {
    commitInfo: GetCommitInfoResponse | undefined;
};

export const CommitInfo: React.FC<CommitInfoProps> = (props) => {
    const commitInfo = props.commitInfo;
    if (commitInfo === undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">Backend returned empty response</main>
            </div>
        );
    }
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
                            <td>{commitInfo.commitHash}</td>
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
                <h2>Events</h2>
                <CommitInfoEvents events={commitInfo.events} />
            </main>
        </div>
    );
};

const CommitInfoEvents: React.FC<{ events: Event[] }> = (props) => (
    <table border={1}>
        <thead>
            <tr>
                <th>Date:</th>
                <th>Event Description:</th>
                <th>Environments:</th>
            </tr>
        </thead>
        <tbody>
            {props.events.map((event) => {
                const createdAt = event.createdAt?.toISOString() || '';
                const [description, environments] = eventDescription(event);
                return (
                    <tr>
                        <td>{createdAt}</td>
                        <td>{description}</td>
                        <td>{environments}</td>
                    </tr>
                );
            })}
        </tbody>
    </table>
);

const eventDescription = (event: Event): [JSX.Element, string] => {
    const tp = event.eventType;
    if (tp === undefined) {
        return [<span>Unspecified event type</span>, ''];
    }
    switch (tp.$case) {
        case 'createReleaseEvent':
            return [
                <span>Kuberpult received data about this commit for the first time</span>,
                tp.createReleaseEvent.environmentNames.join(', '),
            ];
        case 'deploymentEvent':
            const de = tp.deploymentEvent;
            let description: JSX.Element;
            if (de.releaseTrainSource === undefined)
                description = (
                    <span>
                        Manual deployment of application <b>{de.application}</b> to environment{' '}
                        <b>{de.targetEnvironment}</b>
                    </span>
                );
            else {
                if (de.releaseTrainSource?.targetGroup === undefined)
                    description = (
                        <span>
                            Release train deployment of application <b>{de.application}</b> from environment{' '}
                            <b>{de.releaseTrainSource.upstreamEnvironment}</b> to environment{' '}
                            <b>{de.targetEnvironment}</b>
                        </span>
                    );
                else
                    description = (
                        <span>
                            Release train deployment of application <b>{de.application}</b> on environment group{' '}
                            <b>{de.releaseTrainSource.targetGroup}</b> from environment{' '}
                            <b>{de.releaseTrainSource?.upstreamEnvironment}</b> to environment{' '}
                            <b>{de.targetEnvironment}</b>
                        </span>
                    );
            }
            return [description, de.targetEnvironment];
    }
};
