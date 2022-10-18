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

import { Textfield } from '../textfield';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { SideBar } from '../SideBar/SideBar';
import { Button } from '../button';
import { ShowBarWhite } from '../../../images';
import { useSearchParams } from 'react-router-dom';
import { Dropdown } from '../dropdown/dropdown';

export const TopAppBar: React.FC = () => {
    const control = useRef<HTMLDivElement>(null);
    const MDComponent = useRef<MDCTopAppBar>();
    const [sideBar, showSideBar] = useState(false);
    const [params] = useSearchParams();

    const toggleSideBar = useCallback(() => showSideBar((old) => !old), [showSideBar]);

    const query = params.get('application') || undefined;

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
                    <Textfield
                        className={'top-app-bar-search-field'}
                        floatingLabel={'Application Name'}
                        value={query}
                        leadingIcon={'search'}
                    />
                    <Dropdown className={'top-app-bar-search-field'} floatingLabel={'Teams'} leadingIcon={'search'} />
                </div>
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                    <strong>{'Planned Actions'}</strong>
                    <Button className="mdc-show-button" icon={<ShowBarWhite />} onClick={toggleSideBar} />
                    <SideBar
                        className={
                            `mdc-drawer-sidebar mdc-drawer-sidebar-container mdc-drawer-sidebar` +
                            (sideBar ? '--displayed' : '--hidden')
                        }
                        toggleSidebar={toggleSideBar}
                    />
                </div>
            </div>
        </div>
    );
};
