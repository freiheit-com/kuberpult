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
import classNames from 'classnames';
import { Release } from '../../../api/api';
import { useReleasesForApp } from '../../utils/store';
import { ReleaseCardMini } from '../ReleaseCardMini/ReleaseCardMini';
import './Releases.scss';

export type ReleasesProps = {
    className?: string;
    app: string;
};

const dateFormat = (date: Date): string => {
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return `${months[date.getMonth()]} ${date.getDate()}, ${date.getFullYear()}`;
};

const getReleasesForAppGroupByDate = (releases: Array<Release>): Release[][] => {
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
            <h1 className={classNames('app_name', className)}>{'Releases | ' + app}</h1>
            {rel.map((release) => (
                <div key={release[0].version} className={classNames('container right', className)}>
                    <div className={classNames('release_date', className)}>{dateFormat(release[0].createdAt)}</div>
                    {release.map((rele) => (
                        <div key={rele.version} className={classNames('content', className)}>
                            <ReleaseCardMini app={app} version={rele.version} />
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
};
