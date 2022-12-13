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

//import { useReleasesForApp } from '../../utils/store';

export const ReleasesPage: React.FC = () => {
    const url = window.location.href.split('/');
    const app_name = url[url.length - 1];
    /*const releases = useReleasesForApp(app_name);
    const timeLine = (
        <div className="timeline">
            <div className="container left">
                <div className="content">
                    <h2>2017</h2>
                    <p>Lorem ipsum..</p>
                </div>
            </div>
            <div className="container right">
                <div className="content">
                    <h2>2016</h2>
                    <p>Lorem ipsum..</p>
                </div>
            </div>
        </div>
    );*/
    return (
        <main className="main-content">
            <h1 className={classNames('service__name')}>{app_name}</h1>
        </main>
    );
};
