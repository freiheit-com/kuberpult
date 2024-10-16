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
    addAction,
    EnvironmentGroupExtended,
    getAppDetails,
    showSnackbarError,
    showSnackbarWarn,
    useAppDetailsForApp,
    useCurrentlyExistsAtGroup,
    useMinorsForApp,
    useNavigateWithSearchParams,
} from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Environment, GetAppDetailsResponse, OverviewApplication, UndeploySummary } from '../../../api/api';
import * as React from 'react';
import { useCallback, useState } from 'react';
import { AppLockSummary } from '../chip/EnvironmentGroupChip';
import { WarningBoxes } from './Warnings';
import { DotsMenu, DotsMenuButton } from './DotsMenu';
import { EnvSelectionDialog } from '../SelectionDialog/SelectionDialogs';
import { useAzureAuthSub } from '../../utils/AzureAuthProvider';
import { SmallSpinner } from '../Spinner/Spinner';

// number of releases on home. based on design
// we could update this dynamically based on viewport width
const numberOfDisplayedReleasesOnHome = 6;

const getReleasesToDisplay = (
    deployedReleases: number[],
    allReleases: number[],
    minorReleases: number[],
    ignoreMinors: boolean
): number[] => {
    if (ignoreMinors) {
        allReleases = allReleases.filter(
            (version) => !minorReleases.includes(version) || deployedReleases.includes(version)
        );
    }
    // all deployed releases are important and the latest release is also important
    const importantReleases = deployedReleases.includes(allReleases[0])
        ? deployedReleases
        : [allReleases[0], ...deployedReleases];
    // number of remaining releases to get from history
    const numOfTrailingReleases = numberOfDisplayedReleasesOnHome - importantReleases.length;
    // find the index of the last deployed release e.g. Prod (or -1 when there's no deployed releases)
    const oldestImportantReleaseIndex = importantReleases.length
        ? allReleases.findIndex((version) => version === importantReleases.slice(-1)[0])
        : -1;
    // take the deployed releases + a slice from the oldest element (or very first, see above) with total length 6
    return [
        ...importantReleases,
        ...allReleases.slice(oldestImportantReleaseIndex + 1, oldestImportantReleaseIndex + numOfTrailingReleases + 1),
    ];
};

function getNumberOfReleasesBetween(releases: number[], higherVersion: number, lowerVersion: number): number {
    // diff = index of lower version (older release) - index of higher version (newer release) - 1
    return releases.findIndex((ver) => ver === lowerVersion) - releases.findIndex((ver) => ver === higherVersion) - 1;
}

export const DiffElement: React.FC<{ diff: number; title: string; navCallback: () => void }> = ({
    diff,
    title,
    navCallback,
}) => (
    <div
        className="service-lane__diff--container"
        title={title}
        onClick={navCallback}
        data-testid="hidden-commits-button">
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--number">{diff}</div>
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
    </div>
);

const deriveUndeployMessage = (undeploySummary: UndeploySummary | undefined): string | undefined => {
    switch (undeploySummary) {
        case UndeploySummary.UNDEPLOY:
            return 'Delete Forever';
        case UndeploySummary.NORMAL:
            return 'Prepare Undeploy Release';
        case UndeploySummary.MIXED:
            return undefined;
        default:
            return undefined;
    }
};

export const ServiceLane: React.FC<{ application: OverviewApplication; hideMinors: boolean }> = (props) => {
    const { application, hideMinors } = props;
    const { authHeader } = useAzureAuthSub((auth) => auth);

    const appDetails = useAppDetailsForApp(application.name);
    React.useEffect(() => {
        getAppDetails(application.name, authHeader);
    }, [application, authHeader]);

    if (!appDetails) {
        return (
            <div className="service-lane">
                <div className="service-lane__header">
                    <div className="service-lane-wrapper">
                        <div className={'service-lane-name'}>
                            <span title={'team name'}>{application.team ? application.team : '<No Team> '} </span>
                            {' | '} <span title={'app name'}> {application.name}</span>
                        </div>
                        <SmallSpinner appName={application.name} key={application.name} />
                    </div>
                </div>
                <div className="service__releases" key={application.name + '-' + application.team}></div>
            </div>
        );
    }

    return (
        <ReadyServiceLane
            application={application}
            hideMinors={hideMinors}
            appDetails={appDetails}
            key={application.name}></ReadyServiceLane>
    );
};

export const ReadyServiceLane: React.FC<{
    application: OverviewApplication;
    hideMinors: boolean;
    appDetails: GetAppDetailsResponse;
}> = (props) => {
    const { application, hideMinors } = props;
    const { navCallback } = useNavigateWithSearchParams('releasehistory/' + application.name);

    const allReleases = [...new Set(props.appDetails?.application?.releases.map((d) => d.version))].sort(
        (n1, n2) => n2 - n1
    );
    const deployments = props.appDetails?.deployments;
    const allDeployedReleaseNumbers = [];
    for (const prop in deployments) {
        allDeployedReleaseNumbers.push(deployments[prop].version);
    }
    const deployedReleases = [...new Set(allDeployedReleaseNumbers.map((v) => v).sort((n1, n2) => n2 - n1))];

    const prepareUndeployOrUndeploy = React.useCallback(() => {
        switch (props.appDetails.application?.undeploySummary) {
            case UndeploySummary.UNDEPLOY:
                addAction({
                    action: {
                        $case: 'undeploy',
                        undeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.NORMAL:
                addAction({
                    action: {
                        $case: 'prepareUndeploy',
                        prepareUndeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.MIXED:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in mixed state.');
                break;
            default:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in unknown state.');
                break;
        }
    }, [application.name, props.appDetails.application?.undeploySummary]);
    let minorReleases = useMinorsForApp(application.name);
    if (!minorReleases) {
        minorReleases = [];
    }
    const prepareUndeployOrUndeployText = deriveUndeployMessage(props.appDetails.application?.undeploySummary);
    const releases = [
        ...new Set(
            getReleasesToDisplay(deployedReleases, allReleases, minorReleases, hideMinors).sort((n1, n2) => n2 - n1)
        ),
    ];
    const releases_lane =
        !!releases &&
        releases.map((rel, index) => {
            // diff is releases between current card and the next.
            // for the last card, diff is number of remaining releases in history
            const diff =
                index < releases.length - 1
                    ? getNumberOfReleasesBetween(allReleases, rel, releases[index + 1])
                    : getNumberOfReleasesBetween(allReleases, rel, allReleases.slice(-1)[0]) + 1;
            return (
                <div key={application.name + '-' + rel} className="service-lane__diff">
                    <ReleaseCard app={application.name} version={rel} key={application.name + '-' + rel} />
                    {!!diff && (
                        <DiffElement
                            diff={diff}
                            title={'There are ' + diff + ' more releases hidden. Click me to view more.'}
                            navCallback={navCallback}
                        />
                    )}
                </div>
            );
        });

    const envs: Environment[] = useCurrentlyExistsAtGroup(application.name).flatMap(
        (envGroup: EnvironmentGroupExtended) => envGroup.environments
    );

    const [showEnvSelectionDialog, setShowEnvSelectionDialog] = useState(false);

    const handleClose = useCallback(() => {
        setShowEnvSelectionDialog(false);
    }, []);
    const confirmEnvAppDelete = useCallback(
        (selectedEnvs: string[]) => {
            if (selectedEnvs.length === envs.length) {
                showSnackbarWarn("If you want to delete all environments, use 'prepare undeploy'");
                setShowEnvSelectionDialog(false);
                return;
            }
            selectedEnvs.forEach((env) => {
                addAction({
                    action: {
                        $case: 'deleteEnvFromApp',
                        deleteEnvFromApp: { application: application.name, environment: env },
                    },
                });
            });
            setShowEnvSelectionDialog(false);
        },
        [application.name, envs]
    );
    const buttons: DotsMenuButton[] = [
        {
            label: 'View History',
            icon: <HistoryWhite />,
            onClick: (): void => {
                navCallback();
            },
        },
        {
            label: 'Remove environment from app',
            icon: <DeleteWhite />,
            onClick: (): void => {
                setShowEnvSelectionDialog(true);
            },
        },
    ];
    if (prepareUndeployOrUndeployText) {
        buttons.push({
            label: prepareUndeployOrUndeployText,
            onClick: prepareUndeployOrUndeploy,
            icon: <DeleteWhite />,
        });
    }

    const dotsMenu = <DotsMenu buttons={buttons} />;
    const appLocks = Object.values(useAppDetailsForApp(application.name).appLocks);
    const teamLocks = Object.values(useAppDetailsForApp(application.name).teamLocks);
    const dialog = (
        <EnvSelectionDialog
            environments={envs.map((e) => e.name)}
            open={showEnvSelectionDialog}
            onSubmit={confirmEnvAppDelete}
            onCancel={handleClose}
            envSelectionDialog={true}
        />
    );

    return (
        <div className="service-lane">
            {dialog}
            <div className="service-lane__header">
                <div className="service-lane-wrapper">
                    {appLocks.length + teamLocks.length >= 1 && (
                        <div className={'test-app-lock-summary'}>
                            <AppLockSummary app={application.name} numLocks={appLocks.length + teamLocks.length} />
                        </div>
                    )}
                    <div className={'service-lane-name'}>
                        <span title={'team name'}>{application.team ? application.team : '<No Team> '} </span>
                        {' | '} <span title={'app name'}> {application.name}</span>
                    </div>
                </div>
                <div className="service__actions__">{dotsMenu}</div>
            </div>
            <div className="service__warnings">
                <WarningBoxes application={props.appDetails?.application} />
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
