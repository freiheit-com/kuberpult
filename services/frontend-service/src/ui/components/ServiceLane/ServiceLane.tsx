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
import { Application } from '../../../api/api';

export const ServiceLane: React.FC<{ application: Application }> = (props) => {
    const { application } = props;
    const releases = useDeployedReleases(application.name);
    const releases_lane = releases.map((rel, index) => {
        if (index > 0) {
            const diff = releases[index - 1] - rel - 1;
            if (diff !== 0) {
                return (
                    <div key={application + '-' + rel} className="service-lane__diff">
                        <div className="service-lane__diff_number">{diff}</div>
                        <ReleaseCard app={application.name} version={rel} />
                    </div>
                );
            } else {
                return <ReleaseCard app={application.name} version={rel} key={application + '-' + rel} />;
            }
        } else {
            return <ReleaseCard app={application.name} version={rel} key={application + '-' + rel} />;
        }
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
                    />
                </div>
            </div>
            <div className="service__releases">{releases_lane}</div>
        </div>
    );
};
