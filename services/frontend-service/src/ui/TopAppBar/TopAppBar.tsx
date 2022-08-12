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
import { MDCTopAppBar } from '@material/top-app-bar';

import { Button } from '../components/button';
import { Textfield } from '../components/textfield';
import { useEffect, useRef } from 'react';

export const TopAppBar: React.FC = () => {
    const control = useRef<HTMLDivElement>(null);
    const MDComponent = useRef<MDCTopAppBar>();

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCTopAppBar(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <div className="mdc-top-app-bar" ref={control}>
            <div className="mdc-top-app-bar__row">
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-start">
                    <span className="mdc-top-app-bar__title">Kuberpult</span>
                </div>
                <div className="mdc-top-app-bar__section">
                    <Textfield className={'top-app-bar-search-field'} floatingLabel={'Search'} leadingIcon={'search'} />
                </div>
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                    <Button label={'Planned Actions'} />
                </div>
            </div>
        </div>
    );
};
