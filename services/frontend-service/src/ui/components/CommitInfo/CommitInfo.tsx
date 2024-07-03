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
import React, { useState } from 'react';
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
    // most commit messages and with "/n" but that looks odd when rendered in a table:
    const msgWithoutTrailingN = commitInfo.commitMessage.trimEnd();
    const nextPrevMessage =
        'Note that kuberpult links to the next commit in the repository that it is aware of.' +
        'This is not necessarily the next/previous commit that touches the desired microservice.';
    const tooltipMsg =
        ' Limitation: Currently only commits that touch exactly one app are linked. Additionally, kuberpult can only link commits if the previous commit hash is supplied to the /release endpoint.';
    const showInfo = !commitInfo.nextCommitHash || !commitInfo.previousCommitHash;
    const previousButton =
        commitInfo.previousCommitHash !== '' ? (
            <div className="history-button-container">
                <a href={'/ui/commits/' + commitInfo.previousCommitHash} title={nextPrevMessage}>
                    Previous Commit
                </a>
            </div>
        ) : (
            <div className="history-text-container">Previous commit not found &nbsp;</div>
        );
    const nextButton =
        commitInfo.nextCommitHash !== '' ? (
            <div className="history-button-container">
                <a href={'/ui/commits/' + commitInfo.nextCommitHash} title={nextPrevMessage}>
                    Next Commit
                </a>
            </div>
        ) : (
            <div className="history-text-container">Next commit not found &nbsp;</div>
        );
    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content commit-page">
                <h1> Commit: {commitInfo.commitMessage.split('\n')[0]} </h1>
                <div>
                    <table className={'metadata'} border={1}>
                        <thead>
                            <tr>
                                <th className={'hash'}>Commit Hash:</th>
                                <th className={'message'}>Commit Message:</th>
                                <th className={'apps'}>Touched apps:</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>{commitInfo.commitHash}</td>
                                <td>
                                    <div className={'commit-page-message'}>
                                        {msgWithoutTrailingN.split('\n').map((msg, index) => (
                                            <div key={index}>
                                                <span>{msg}</span>
                                                <br />
                                            </div>
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
                                {previousButton}

                                {nextButton}
                                {showInfo && <div title={tooltipMsg}> ⓘ </div>}
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

const CommitInfoEvents: React.FC<{ events: Event[] }> = (props) => {
    const [timezone, setTimezone] = useState<'UTC' | 'local'>('UTC');
    const handleChangeTimezone = React.useCallback(
        (event: React.ChangeEvent<HTMLSelectElement>) => {
            if (event.target.value === 'local' || event.target.value === 'UTC') {
                setTimezone(event.target.value);
            }
        },
        [setTimezone]
    );
    const formatDate = (date: Date | undefined): string => {
        if (!date) return '';
        const selectedTimezone = timezone === 'local' ? Intl.DateTimeFormat().resolvedOptions().timeZone : 'UTC';
        const localizedDate = new Date(
            date.toLocaleString('en-US', {
                timeZone: selectedTimezone,
            })
        );
        return localizedDate.toISOString().split('.')[0];
    };
    return (
        <div>
            <select className={'select-timezone'} value={timezone} onChange={handleChangeTimezone}>
                <option value="local">Local Timezone</option>
                <option value="UTC">UTC Timezone</option>
            </select>
            <table className={'events'} border={1}>
                <thead>
                    <tr>
                        <th className={'date'}>Date:</th>
                        <th className={'description'}>Event Description:</th>
                        <th className={'environments'}>Environments:</th>
                    </tr>
                </thead>
                <tbody>
                    {props.events.map((event, _) => {
                        const createdAt = formatDate(event.createdAt);
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
        </div>
    );
};

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
                // if the releaseTrainSource is undefined, it could be either a
                // manual deployment by the user or
                // an automatic deployment because of the "upstream.latest" configuration of this environment
                description = (
                    <span>
                        Single deployment of application <b>{de.application}</b> to environment{' '}
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
        case LockPreventedDeploymentEvent_LockType.LOCK_TYPE_TEAM:
            return 'a team lock';
        case LockPreventedDeploymentEvent_LockType.LOCK_TYPE_UNKNOWN:
        case LockPreventedDeploymentEvent_LockType.UNRECOGNIZED:
            return 'an unknown lock';
    }
};
