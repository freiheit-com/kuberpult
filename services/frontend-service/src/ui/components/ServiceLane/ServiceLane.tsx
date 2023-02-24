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
import { useDeployedReleases, useVersionsForApp } from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { Button } from '../button';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Application } from '../../../api/api';
import { Tooltip } from '@material-ui/core';
import { useNavigate } from 'react-router-dom';
import * as React from 'react';

// number of releases on home. based on design
const numberOfDisplayedReleasesOnHome = 6;

const getReleasesToDisplay = (deployedReleases: number[], allReleases: number[]): number[] => {
    // number of remaining releases to get from history
    const numOfTrailingReleases = numberOfDisplayedReleasesOnHome - deployedReleases.length;
    // find the index of the last deployed release e.g. Prod
    const oldestDeployedReleaseIndex = deployedReleases.length
        ? allReleases.findIndex((version) => version === deployedReleases.slice(-1)[0])
        : 0;
    // take the deployed releases + a slice from the oldest element with total length 6
    return [
        ...deployedReleases,
        ...allReleases.slice(oldestDeployedReleaseIndex + 1, oldestDeployedReleaseIndex + numOfTrailingReleases + 1),
    ];
};

function getNumberOfReleasesBetween(releases: number[], higherVersion: number, lowerVersion: number): number {
    // = index of lower version (older release) - index of lower version (newer release) - 1
    return releases.findIndex((ver) => ver === lowerVersion) - releases.findIndex((ver) => ver === higherVersion) - 1;
}

export const ServiceLane: React.FC<{ application: Application }> = (props) => {
    const { application } = props;
    const deployedReleases = useDeployedReleases(application.name);
    const allReleases = useVersionsForApp(application.name);
    const navigate = useNavigate();
    const navigateToReleases = React.useCallback(() => {
        navigate('releases/' + application.name);
    }, [application.name, navigate]);
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
                        <Tooltip
                            title={
                                'There are ' +
                                diff +
                                ' releases between version ' +
                                releases[index - 1] +
                                ' and version ' +
                                rel
                            }>
                            <div className="service-lane__diff_number">{diff}</div>
                        </Tooltip>
                    )}
                </div>
            );
        });

    return (
        <div className="service-lane">
            <div className="service-lane__header">
                <div className="service__name">
                    {(application.team ? application.team + ' | ' : '<No Team> | ') + application.name}
                </div>
                <div className="service__actions">
                    <Button
                        className="service-action service-action--prepare-undeploy"
                        label={'Prepare to delete'}
                        icon={<DeleteWhite />}
                    />
                    <Button
                        className="service-action service-action--history"
                        label={'View history'}
                        icon={<HistoryWhite />}
                        onClick={navigateToReleases}
                    />
                </div>
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
