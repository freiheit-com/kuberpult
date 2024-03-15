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
import { GetCommitInfoResponse, Event, LockPreventedDeploymentEvent_LockType } from '../../../api/api';

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
                <div>
                    <table border={1}>
                        <thead>
                            <tr>
                                <th>Commit Hash:</th>
                                <th>Commit Message:</th>
                                <th>Touched apps:</th>
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
                    <div>
                        {commitInfo.touchedApps.length < 2 && (
                            <div className="next-prev-buttons">
                                {commitInfo.previousCommitHash !== '' && (
                                    <div className="history-button-container">
                                        <a href={'/ui/commits/' + commitInfo.previousCommitHash}>Previous Commit</a>
                                    </div>
                                )}

                                {commitInfo.nextCommitHash !== '' && (
                                    <div className="history-button-container">
                                        <a href={'/ui/commits/' + commitInfo.nextCommitHash}>Next Commit</a>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
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
            {props.events.map((event, eventIdx) => {
                const createdAt = event.createdAt?.toISOString() || '';
                const [description, environments] = eventDescription(event);
                return (
                    <tr key={event.uuid}>
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
                if (de.releaseTrainSource?.targetEnvironmentGroup === undefined)
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
                            <b>{de.releaseTrainSource.targetEnvironmentGroup}</b> from environment{' '}
                            <b>{de.releaseTrainSource?.upstreamEnvironment}</b> to environment{' '}
                            <b>{de.targetEnvironment}</b>
                        </span>
                    );
            }
            return [description, de.targetEnvironment];
        case 'lockPreventedDeploymentEvent':
            const inner = tp.lockPreventedDeploymentEvent;
            return [
                <span>
                    Application <b>{inner.application}</b> was blocked from deploying due to{' '}
                    {lockTypeName(inner.lockType)} with message "{inner.lockMessage}"
                </span>,
                inner.environment,
            ];
        case 'replacedByEvent':
            return [
                <span>
                    This commit was replaced by{' '}
                    <a href={'/ui/commits/' + tp.replacedByEvent.replacedByCommitId}>
                        {tp.replacedByEvent.replacedByCommitId.substring(0, 8)}
                    </a>{' '}
                    on <b>{tp.replacedByEvent.environment}</b>.
                </span>,
                tp.replacedByEvent.environment,
            ];
    }
};

const lockTypeName = (tp: LockPreventedDeploymentEvent_LockType): string => {
    switch (tp) {
        case LockPreventedDeploymentEvent_LockType.LOCK_TYPE_APP:
            return 'an application lock';
        case LockPreventedDeploymentEvent_LockType.LOCK_TYPE_ENV:
            return 'an environment lock';
        case LockPreventedDeploymentEvent_LockType.LOCK_TYPE_UNKNOWN:
        case LockPreventedDeploymentEvent_LockType.UNRECOGNIZED:
            return 'an unknown lock';
    }
};
