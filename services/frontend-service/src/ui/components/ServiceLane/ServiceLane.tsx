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
import { Button } from '../button';
import { DeleteWhite, HistoryWhite } from '../../../images';

export const ServiceLane: React.FC<{ application: string }> = (props) => {
    const { application } = props;
    const releases = useDeployedReleases(application);
    return (
        <div className="service-lane">
            <div className="service-lane__header">
                <div className="service__name">{application}</div>
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
                    />
                </div>
            </div>
            <div className="service__releases">
                {releases.map((rel) => (
                    <ReleaseCard app={application} version={rel} key={application + '-' + rel} />
                ))}
            </div>
        </div>
    );
};
