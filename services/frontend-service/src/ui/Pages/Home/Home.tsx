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
import { useAllApplicationNames } from '../../utils/store';
import { ApplicationRow } from '../../components/ApplicationRow/ApplicationRow';

export const Home: React.FC = () => {
    const apps = useAllApplicationNames();
    apps.sort((a, b) => a.localeCompare(b));
    return (
        <main className="main-content">
            {apps.map((app) => (
                <ApplicationRow app={app} />
            ))}
        </main>
    );
};
