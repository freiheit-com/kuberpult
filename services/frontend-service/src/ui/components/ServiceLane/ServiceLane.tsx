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
import { useDeployedReleases } from '../../utils/store';
import { ReleaseCard } from '../ReleaseCard/ReleaseCard';

export const ServiceLane: React.FC<{ application: string }> = (props) => {
    const { application } = props;
    const releases = useDeployedReleases(application);
    return (
        <div>
            <h1>{application}</h1>
            <div className="service-releases">
                {releases.map((rel) => (
                    <ReleaseCard app={application} version={rel} key={application + '-' + rel} />
                ))}
            </div>
        </div>
    );
};
