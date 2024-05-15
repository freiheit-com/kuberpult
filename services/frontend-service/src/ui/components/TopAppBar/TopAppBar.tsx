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

Copyright freiheit.com*/
import { Textfield } from '../textfield';
import React, { useCallback } from 'react';
import { SideBar } from '../SideBar/SideBar';
import { Button } from '../button';
import { ShowBarWhite } from '../../../images';
import { useSearchParams } from 'react-router-dom';
import { Dropdown } from '../dropdown/dropdown';
import { Checkbox } from '../dropdown/checkbox';
import classNames from 'classnames';
import {
    UpdateSidebar,
    useAllWarnings,
    useKuberpultVersion,
    useShownWarnings,
    useSidebarShown,
} from '../../utils/store';
import { Warning } from '../../../api/api';
import { hideWithoutWarnings, KuberpultGitHubLink, setHideWithoutWarnings } from '../../utils/Links';

export type TopAppBarProps = {
    showAppFilter: boolean;
    showTeamFilter: boolean;
    showWarningFilter: boolean;
};

export const TopAppBar: React.FC<TopAppBarProps> = (props) => {
    const sideBar = useSidebarShown();
    const [params, setParams] = useSearchParams();

    const toggleSideBar = useCallback(() => UpdateSidebar.set({ shown: !sideBar }), [sideBar]);
    const appNameParam = params.get('application') || '';
    const teamsParam = (params.get('teams') || '').split(',').filter((val) => val !== '');

    const version = useKuberpultVersion() || '2.6.0';

    const hideWithoutWarningsValue = hideWithoutWarnings(params);
    const allWarnings: Warning[] = useAllWarnings();
    const shownWarnings: Warning[] = useShownWarnings(teamsParam, appNameParam);

    const onWarningsFilterClick = useCallback((): void => {
        setHideWithoutWarnings(params, !hideWithoutWarningsValue);
        setParams(params);
    }, [hideWithoutWarningsValue, params, setParams]);

    const renderedWarnings =
        allWarnings.length === 0 ? (
            ''
        ) : (
            <div className="service-lane__warning">
                {shownWarnings.length} warnings shown ({allWarnings.length} total).
            </div>
        );

    const [searchParams, setSearchParams] = useSearchParams(
        appNameParam === '' ? undefined : { application: `${appNameParam}` }
    );
    const onChangeApplication = useCallback(
        (event: any) => {
            if (event.target.value !== '') searchParams.set('application', event.target.value);
            else searchParams.delete('application');
            setSearchParams(searchParams);
        },
        [searchParams, setSearchParams]
    );

    const renderedAppFilter =
        props.showAppFilter === true ? (
            <div className="mdc-top-app-bar__section top-app-bar--wide-filter">
                <Textfield
                    className={'top-app-bar-search-field'}
                    placeholder={'Application Name'}
                    value={appNameParam}
                    leadingIcon={'search'}
                    onChange={onChangeApplication}
                />
            </div>
        ) : (
            <div className="mdc-top-app-bar__section top-app-bar--wide-filter"></div>
        );
    const renderedTeamsFilter =
        props.showTeamFilter === true ? (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter">
                <Dropdown className={'top-app-bar-search-field'} placeholder={'Teams'} leadingIcon={'search'} />
            </div>
        ) : (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter"></div>
        );
    const renderedWarningsFilter =
        props.showWarningFilter === true ? (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter">
                <Checkbox
                    enabled={hideWithoutWarningsValue}
                    onClick={onWarningsFilterClick}
                    id="warningFilter"
                    label="hide apps without warnings"
                    classes=""
                />
            </div>
        ) : (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter"></div>
        );

    return (
        <div className="mdc-top-app-bar">
            <div className="mdc-top-app-bar__row">
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-start">
                    <span className="mdc-top-app-bar__title">
                        Kuberpult <KuberpultGitHubLink version={version} />
                    </span>
                </div>
                {renderedAppFilter}
                {renderedTeamsFilter}
                {renderedWarningsFilter}
                {renderedWarnings}
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                    <strong className="sub-headline1">Planned Actions</strong>
                    <Button
                        className="mdc-show-button mdc-button--unelevated"
                        icon={<ShowBarWhite />}
                        onClick={toggleSideBar}
                        highlightEffect={false}
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
