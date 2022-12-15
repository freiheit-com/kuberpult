/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/

import classNames from 'classnames';
import { Release } from '../../../api/api';
import { useReleasesForApp } from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import './Releases.scss';

export type ReleasesProps = {
    className?: string;
    app: string;
};

const dateFormat = (date: Date) => {
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return `${months[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()}`;
};

const getReleasesForAppGroupByDate = (releases: Array<Release>) => {
    if (releases === undefined) {
        return [];
    }
    const releaseGroupedByCreatedAt = releases.reduce((previousRelease: Release, curRelease: Release) => {
        (previousRelease[curRelease.createdAt?.toDateString()] =
            previousRelease[curRelease.createdAt?.toDateString()] || []).push(curRelease);
        return previousRelease;
    }, {});
    const rel: Array<Array<Release>> = [];
    for (const [, value] of Object.entries(releaseGroupedByCreatedAt)) {
        rel.push(value);
    }
    return rel;
};

export const Releases: React.FC<ReleasesProps> = (props) => {
    const { app, className } = props;
    const releases = useReleasesForApp(app);
    const rel = getReleasesForAppGroupByDate(releases);

    return (
        <div className={classNames('timeline', className)}>
            <h1 className={classNames('app_name', className)}>{app}</h1>
            {rel.map((release) => (
                <div key={release[0].version} className={classNames('container right', className)}>
                    <div className={classNames('release_date', className)}>{dateFormat(release[0].createdAt)}</div>
                    {release.map((rele) => (
                        <div key={rele.version} className={classNames('content', className)}>
                            <ReleaseCard app={app} version={rele.version} />
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
};
