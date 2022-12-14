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
import { useReleasesForApp } from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';
import './Releases.scss';

export type ReleasesProps = {
    className?: string;
    app: string;
};

export const Releases: React.FC<ReleasesProps> = (props) => {
    const { app, className } = props;

    const releases = useReleasesForApp(app);

    if (releases === undefined) {
        return <div>No releases for app {app}</div>;
    }
    return (
        <div className={classNames('timeline', className)}>
            <h1 className={classNames('app_name', className)}>{app}</h1>
            {releases.map((release) => (
                <div className={classNames('container', className)}>
                    <div className={classNames('content', className)}>
                        <ReleaseCard app={app} version={release.version} />
                    </div>
                </div>
            ))}
        </div>
    );
};
