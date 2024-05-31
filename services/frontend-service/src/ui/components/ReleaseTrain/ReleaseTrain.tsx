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
    GetReleaseTrainPrognosisResponse,
    ReleaseTrainAppPrognosis,
    ReleaseTrainAppSkipCause,
    ReleaseTrainEnvPrognosis_AppsPrognosesWrapper,
    ReleaseTrainEnvSkipCause,
} from '../../../api/api';
import { useRelease } from '../../utils/store';
import { TopAppBar } from '../TopAppBar/TopAppBar';

type ReleaseTrainProps = {
    releaseTrainPrognosis: GetReleaseTrainPrognosisResponse | undefined;
};

export const ReleaseTrain: React.FC<ReleaseTrainProps> = (props) => {
    const releaseTrainPrognosis = props.releaseTrainPrognosis;

    if (releaseTrainPrognosis === undefined) {
        return (
            <div>
                <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
                <main className="main-content commit-page">Backend returned empty response</main>
            </div>
        );
    }

    const envPrognoses = releaseTrainPrognosis.envsPrognoses;

    if (Object.keys(envPrognoses).length === 0) {
        return <h1>Release train is empty</h1>;
    }

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content commit-page">
                {Object.entries(envPrognoses).map(([envName, envPrognosis]) => {
                    const header = <h1>Prognosis for release train on environment {envName}</h1>;
                    const outcome = envPrognosis.outcome;

                    let content: JSX.Element = <div></div>;
                    if (outcome === undefined) {
                        content = (
                            <p>
                                Error retrieving the prognosis for this environment: backend returned undefined value.
                            </p>
                        );
                    } else {
                        if (outcome.$case === 'skipCause') {
                            content = <EnvironmentPrognosisOutcomeSkipped skipCause={outcome.skipCause} />;
                        } else {
                            content = (
                                <EnvironmentPrognosisOutcomeApplicationPrognoses
                                    appsPrognoses={outcome.appsPrognoses}
                                />
                            );
                        }
                    }

                    return (
                        <div>
                            {header}
                            {content}
                        </div>
                    );
                })}
            </main>
        </div>
    );
};

const EnvironmentPrognosisOutcomeSkipped: React.FC<{ skipCause: ReleaseTrainEnvSkipCause }> = ({ skipCause }) => {
    switch (skipCause) {
        case ReleaseTrainEnvSkipCause.ENV_IS_LOCKED:
            return <p>Release train on this environment is skipped because it is locked.</p>;
        case ReleaseTrainEnvSkipCause.ENV_HAS_BOTH_UPSTREAM_LATEST_AND_UPSTREAM_ENV:
            return (
                <p>
                    Release train on this environment is skipped because it both has an upstream environment and is set
                    as latest.
                </p>
            );

        case ReleaseTrainEnvSkipCause.ENV_HAS_NO_UPSTREAM:
            return <p>Release train on this environment is skipped because it has no upstream configured.</p>;

        case ReleaseTrainEnvSkipCause.ENV_HAS_NO_UPSTREAM_LATEST_OR_UPSTREAM_ENV:
            return (
                <p>
                    Release train on this environment is skipped because neither it has an upstream environment
                    configured nor is marked as latest.
                </p>
            );

        case ReleaseTrainEnvSkipCause.UPSTREAM_ENV_CONFIG_NOT_FOUND:
            return (
                <p>
                    Release train on this environment is skipped because no configuration was found for it in the
                    manifest repository.
                </p>
            );

        case ReleaseTrainEnvSkipCause.UNRECOGNIZED:
            return <p>Release train on this environment is skipped due to an unknown reason.</p>;

        default:
            return <p>Release train on this environment is skipped due to an unknown reason.</p>;
    }
};

const EnvironmentPrognosisOutcomeApplicationPrognoses: React.FC<{
    appsPrognoses: ReleaseTrainEnvPrognosis_AppsPrognosesWrapper;
}> = ({ appsPrognoses }) => (
    <table>
        <thead>
            <tr>
                <td>Application</td>
                <td>Outcome</td>
            </tr>
        </thead>
        <tbody>
            {Object.entries(appsPrognoses.prognoses).map(([appName, appPrognosis]) => (
                <ApplicationPrognosisRow appName={appName} appPrognosis={appPrognosis} />
            ))}
        </tbody>
    </table>
);

const ApplicationPrognosisOutcomeReleaseCell: React.FC<{ appName: string; version: number }> = (props) => {
    const release = useRelease(props.appName, props.version);
    if (release === undefined) {
        return (
            <p>
                Commit <i>loading</i> will be released.
            </p>
        );
    }
    return <p>Commit {release.sourceCommitId} will be released.</p>;
};

const ApplicationPrognosisOutcomeSkipCell: React.FC<{ skipCause: ReleaseTrainAppSkipCause }> = ({ skipCause }) => {
    switch (skipCause) {
        case ReleaseTrainAppSkipCause.APP_ALREADY_IN_UPSTREAM_VERSION:
            return <p>Application release is skipped because it is already in the upstream version.</p>;
        case ReleaseTrainAppSkipCause.APP_DOES_NOT_EXIST_IN_ENV:
            return <p>Application release is skipped because it does not exist in the environment.</p>;
        case ReleaseTrainAppSkipCause.APP_HAS_NO_VERSION_IN_UPSTREAM_ENV:
            return (
                <p>Application release is skipped because it does not have a version in the upstream environment.</p>
            );
        case ReleaseTrainAppSkipCause.APP_IS_LOCKED:
            return <p>Application release is skipped because it is locked.</p>;
        case ReleaseTrainAppSkipCause.APP_IS_LOCKED_BY_ENV:
            return (
                <p>
                    Application release is skipped because there's an environment lock where this application is getting
                    deployed.
                </p>
            );
        case ReleaseTrainAppSkipCause.TEAM_IS_LOCKED:
            return <p>Application release is skipped due to a team lock</p>;
        case ReleaseTrainAppSkipCause.UNRECOGNIZED:
            return <p>Application release it skipped due to an unrecognized reason</p>;
        default:
            return <p>Universe on fire</p>;
    }
};

const ApplicationPrognosisRow: React.FC<{ appName: string; appPrognosis: ReleaseTrainAppPrognosis }> = ({
    appName,
    appPrognosis,
}) => {
    let outcomeCell: React.ReactNode;
    const outcome = appPrognosis.outcome;
    if (outcome === undefined) {
        outcomeCell = <p>Error retrieving the outcome of application: backend returned undefined value.</p>;
    } else {
        if (outcome.$case === 'skipCause') {
            outcomeCell = <ApplicationPrognosisOutcomeSkipCell skipCause={outcome.skipCause} />;
        } else {
            outcomeCell = (
                <ApplicationPrognosisOutcomeReleaseCell appName={appName} version={outcome.deployedVersion} />
            );
        }
    }

    return (
        <tr>
            <td>{appName}</td>
            <td>{outcomeCell}</td>
        </tr>
    );
};
