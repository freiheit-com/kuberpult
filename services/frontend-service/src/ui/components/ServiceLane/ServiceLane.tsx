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
import {
    addAction,
    showSnackbarError,
    useDeployedReleases,
    useFilteredApplicationLocks,
    useNavigateWithSearchParams,
    useVersionsForApp,
} from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { Button } from '../button';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Application, UndeploySummary } from '../../../api/api';
import { Tooltip } from '@material-ui/core';
import * as React from 'react';
import { AppLockSummary } from '../chip/EnvironmentGroupChip';

// number of releases on home. based on design
// we could update this dynamically based on viewport width
const numberOfDisplayedReleasesOnHome = 6;

const getReleasesToDisplay = (deployedReleases: number[], allReleases: number[]): number[] => {
    // all deployed releases are important, latest release is also important
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

const DiffElement = (diff: number): JSX.Element => (
    <div className="service-lane__diff--container">
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--number">{diff}</div>
        <div className="service-lane__diff--dot" />
        <div className="service-lane__diff--dot" />
    </div>
);

const deriveUndeployMessage = (undeploySummary: UndeploySummary): string | undefined => {
    switch (undeploySummary) {
        case UndeploySummary.Undeploy:
            return 'Delete Forever';
        case UndeploySummary.Normal:
            return 'Prepare Undeploy Release';
        case UndeploySummary.Mixed:
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
            case UndeploySummary.Undeploy:
                addAction({
                    action: {
                        $case: 'undeploy',
                        undeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.Normal:
                addAction({
                    action: {
                        $case: 'prepareUndeploy',
                        prepareUndeploy: { application: application.name },
                    },
                });
                break;
            case UndeploySummary.Mixed:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in mixed state.');
                break;
            default:
                showSnackbarError('Internal Error: Cannot prepare to undeploy or actual undeploy in unknown state.');
                break;
        }
    }, [application.name, application.undeploySummary]);
    const releases = getReleasesToDisplay(deployedReleases, allReleases);

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
                        <Tooltip title={'There are ' + diff + ' more releases hidden. Click on History to view more'}>
                            {DiffElement(diff)}
                        </Tooltip>
                    )}
                </div>
            );
        });

    const undeployButton = prepareUndeployOrUndeployText ? (
        <Button
            className="service-action service-action--prepare-undeploy mdc-button--unelevated"
            label={prepareUndeployOrUndeployText}
            icon={<DeleteWhite />}
            onClick={prepareUndeployOrUndeploy}
        />
    ) : null;

    const appLocks = useFilteredApplicationLocks(application.name);
    return (
        <div className="service-lane">
            <div className="service-lane__header">
                <div className="service__name">
                    {(application.team ? application.team + ' | ' : '<No Team> | ') + application.name}
                    {appLocks.length >= 1 && (
                        <div className={'test-app-lock-summary'}>
                            <AppLockSummary app={application.name} numLocks={appLocks.length} />
                        </div>
                    )}
                </div>
                <div className="service__actions">
                    {undeployButton}
                    <Button
                        className="service-action service-action--history mdc-button--unelevated"
                        label={'View history'}
                        icon={<HistoryWhite />}
                        onClick={navCallback}
                    />
                </div>
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
