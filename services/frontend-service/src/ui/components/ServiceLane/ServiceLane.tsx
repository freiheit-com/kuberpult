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
import { useDeployedReleases, useReleasesForApp } from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import { Button } from '../button';
import { DeleteWhite, HistoryWhite } from '../../../images';
import { Application, Release } from '../../../api/api';
import { Tooltip } from '@material-ui/core';
import { useNavigate } from 'react-router-dom';
import * as React from 'react';

// number of releases on home. based on design
const numberOfDisplayedReleasesOnHome = 6;

const getAllDisplayedReleases = (deployedReleases: number[], allReleases: number[]): number[] => {
    // number of remaining releases to get from history
    const numOfTrailingReleases = numberOfDisplayedReleasesOnHome - deployedReleases.length;
    // last deployed release e.g. Prod
    const oldestDeployedRelease = deployedReleases[deployedReleases.length - 1];

    const allDisplayedReleases = deployedReleases;
    // go over the remaining spots and fill from history
    for (let i = 1; i <= numOfTrailingReleases; i++) {
        const index = allReleases.indexOf(oldestDeployedRelease) + i;
        if (index >= allReleases.length) break;
        allDisplayedReleases.push(allReleases[index]);
    }
    return allDisplayedReleases;
};

function getNumberOfReleasesBetween(releases: Array<Release>, lowerVersion: number, higherVersion: number): number {
    let diff = 0;
    let count = false;
    if (!releases) {
        return diff;
    }

    for (const release of releases) {
        if (release.version === higherVersion) {
            break;
        }
        if (count) {
            diff += 1;
        }
        if (release.version === lowerVersion) {
            count = true;
        }
    }
    return diff;
}

export const ServiceLane: React.FC<{ application: Application }> = (props) => {
    const { application } = props;
    const deployedReleases = useDeployedReleases(application.name);
    const all_releases = useReleasesForApp(application.name);
    const navigate = useNavigate();
    const navigateToReleases = React.useCallback(() => {
        navigate('releases/' + application.name);
    }, [application.name, navigate]);
    const releases = getAllDisplayedReleases(
        deployedReleases,
        all_releases.map((rel) => rel.version)
    );

    const releases_lane =
        !!releases &&
        releases.map((rel, index) => {
            const diff = index > 0 ? getNumberOfReleasesBetween(all_releases, releases[index - 1], rel) : 0;
            return (
                <div key={application + '-' + rel} className="service-lane__diff">
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
                    <ReleaseCard app={application.name} version={rel} key={application + '-' + rel} />
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
