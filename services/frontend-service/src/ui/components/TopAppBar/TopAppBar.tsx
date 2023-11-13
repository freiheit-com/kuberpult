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
import { MDCTopAppBar } from '@material/top-app-bar';

import { Textfield } from '../textfield';
import React, { useCallback, useEffect, useRef } from 'react';
import { SideBar } from '../SideBar/SideBar';
import { Button } from '../button';
import { ShowBarWhite } from '../../../images';
import { useSearchParams } from 'react-router-dom';
import { Dropdown } from '../dropdown/dropdown';
import classNames from 'classnames';
import { UpdateSidebar, useAllWarnings, useKuberpultVersion, useSidebarShown } from '../../utils/store';
import { Warning } from '../../../api/api';

export type TopAppBarProps = {
    showAppFilter: boolean;
    showTeamFilter: boolean;
};

export const TopAppBar: React.FC<TopAppBarProps> = (props) => {
    const control = useRef<HTMLDivElement>(null);
    const MDComponent = useRef<MDCTopAppBar>();
    const sideBar = useSidebarShown();
    const [params] = useSearchParams();

    const toggleSideBar = useCallback(() => UpdateSidebar.set({ shown: !sideBar }), [sideBar]);
    const query = params.get('application') || undefined;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCTopAppBar(control.current);
        }
        return (): void => MDComponent.current?.destroy();
    }, []);

    const version = useKuberpultVersion();

    const allWarnings: Warning[] = useAllWarnings();
    const renderedWarnings =
        allWarnings.length === 0 ? (
            ''
        ) : (
            <div className="service-lane__warning">There are {allWarnings.length} warnings total.</div>
        );

    const renderedAppFilter =
        props.showAppFilter === true ? (
            <div className="mdc-top-app-bar__section">
                <Textfield
                    className={'top-app-bar-search-field'}
                    floatingLabel={'Application Name'}
                    value={query}
                    leadingIcon={'search'}
                />
            </div>
        ) : (
            ''
        );
    const renderedTeamsFilter =
        props.showTeamFilter === true ? (
            <div className="mdc-top-app-bar__section">
                <Dropdown className={'top-app-bar-search-field'} floatingLabel={'Teams'} leadingIcon={'search'} />
            </div>
        ) : (
            ''
        );
    return (
        <div className="mdc-top-app-bar" ref={control}>
            <div className="mdc-top-app-bar__row">
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-start">
                    <span className="mdc-top-app-bar__title">Kuberpult v{version}</span>
                </div>
                <div className="mdc-top-app-bar__section">{renderedWarnings}</div>
                {renderedAppFilter}
                {renderedTeamsFilter}
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                    <strong className="sub-headline1">Planned Actions</strong>
                    <Button
                        className="mdc-show-button mdc-button--unelevated"
                        icon={<ShowBarWhite />}
                        onClick={toggleSideBar}
                    />
                    <SideBar
                        className={classNames(`mdc-drawer-sidebar mdc-drawer-sidebar-container`, {
                            'mdc-drawer-sidebar--displayed': sideBar,
                            'mdc-drawer-sidebar--hidden': !sideBar,
                        })}
                        toggleSidebar={toggleSideBar}
                    />
                </div>
            </div>
        </div>
    );
};
