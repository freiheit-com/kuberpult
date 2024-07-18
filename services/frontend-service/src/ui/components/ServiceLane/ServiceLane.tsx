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
    showSnackbarError,
    showSnackbarWarn,
    useCurrentlyExistsAtGroup,
    useDeployedReleases,
    useFilteredApplicationLocks,
    useNavigateWithSearchParams,
    useTeamLocksFilterByTeam,
    useVersionsForApp,
} from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Application, Environment, UndeploySummary } from '../../../api/api';
import * as React from 'react';
import { AppLockSummary, TeamLockSummary } from '../chip/EnvironmentGroupChip';
import { WarningBoxes } from './Warnings';
import { DotsMenu, DotsMenuButton } from './DotsMenu';
import { useCallback, useState } from 'react';
import { EnvSelectionDialog } from '../SelectionDialog/SelectionDialogs';

// number of releases on home. based on design
// we could update this dynamically based on viewport width
const numberOfDisplayedReleasesOnHome = 6;

const getReleasesToDisplay = (deployedReleases: number[], allReleases: number[]): number[] => {
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

const DiffElement: React.FC<{ diff: number; title: string }> = ({ diff, title }) => (
    <div className="service-lane__diff--container" title={title}>
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--number">{diff}</div>
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
    </div>
);

const deriveUndeployMessage = (undeploySummary: UndeploySummary): string | undefined => {
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

export const ServiceLane: React.FC<{ application: Application }> = (props) => {
    const { application } = props;
    const deployedReleases = useDeployedReleases(application.name);
    const allReleases = useVersionsForApp(application.name);
    const { navCallback } = useNavigateWithSearchParams('releases/' + application.name);
    const prepareUndeployOrUndeployText = deriveUndeployMessage(application.undeploySummary);

    const prepareUndeployOrUndeploy = React.useCallback(() => {
        switch (application.undeploySummary) {
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
    }, [application.name, application.undeploySummary]);
    const releases = getReleasesToDisplay(deployedReleases, allReleases);

    if (application.name === 'abc') {
        // eslint-disable-next-line no-console
        console.log('AllReleases:');
        // eslint-disable-next-line no-console
        console.log(releases);
        // eslint-disable-next-line no-console
        console.log('deployedReleases:');
        // eslint-disable-next-line no-console
        console.log(deployedReleases);
        // eslint-disable-next-line no-console
        console.log('releases:');
        // eslint-disable-next-line no-console
        console.log(releases);
    }
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
                            title={'There are ' + diff + ' more releases hidden. Click on History to view more'}
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
    const appLocks = useFilteredApplicationLocks(application.name);
    const teamLocks = useTeamLocksFilterByTeam(application.team);
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
                <div className="service__name">
                    {application.team ? application.team : '<No Team> '}
                    {teamLocks.length >= 1 && (
                        <div className={'test-app-lock-summary'}>
                            <TeamLockSummary team={application.team} numLocks={teamLocks.length} />
                        </div>
                    )}
                    {' | ' + application.name}
                    {appLocks.length >= 1 && (
                        <div className={'test-app-lock-summary'}>
                            <AppLockSummary app={application.name} numLocks={appLocks.length} />
                        </div>
                    )}
                </div>
                <div className="service__actions__">{dotsMenu}</div>
            </div>
            <div className="service__warnings">
                <WarningBoxes application={application} />
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
