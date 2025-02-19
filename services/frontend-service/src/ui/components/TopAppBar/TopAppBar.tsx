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
import { jwtDecode } from 'jwt-decode';
import { Textfield } from '../textfield';
import React, { useCallback } from 'react';

import { useSearchParams } from 'react-router-dom';
import { TeamsFilterDropdown, FiltersDropdown } from '../dropdown/dropdown';
import classNames from 'classnames';
import {
    applicationsWithWarnings,
    useAllWarnings,
    useAllWarningsAllApps,
    useApplicationsFilteredAndSorted,
    useKuberpultVersion,
} from '../../utils/store';
import { Warning } from '../../../api/api';
import {
    hideMinors,
    setHideMinors,
    hideWithoutWarnings,
    KuberpultGitHubLink,
    setHideWithoutWarnings,
} from '../../utils/Links';
import { GeneralGitSyncStatus } from '../GeneralGitSyncStatus/GeneralSyncStatus';
import { SideBar } from '../SideBar/SideBar';

export type TopAppBarProps = {
    showAppFilter: boolean;
    showTeamFilter: boolean;
    showWarningFilter: boolean;
    showGitSyncStatus: boolean;
};

export const TopAppBar: React.FC<TopAppBarProps> = (props) => {
    const [params, setParams] = useSearchParams();
    useAllWarningsAllApps();
    const appNameParam = params.get('application') || '';
    const teamsParam = (params.get('teams') || '').split(',').filter((val) => val !== '');

    const version = useKuberpultVersion() || '2.6.0';
    const cookieValue = document.cookie
        .split('; ')
        .find((row) => row.startsWith('kuberpult.oauth='))
        ?.split('=')[1];
    const decodedToken: any = cookieValue ? jwtDecode(cookieValue) : undefined;
    const loggedInUser = decodedToken?.email || 'Guest';

    const hideWithoutWarningsValue = hideWithoutWarnings(params);

    const allWarnings: Warning[] = useAllWarnings();

    const shownApps = useApplicationsFilteredAndSorted(teamsParam, true, appNameParam);

    const ShownAppsWithWarnings = applicationsWithWarnings(shownApps);

    const hideMinorsValue = hideMinors(params);

    const onWarningsFilterClick = useCallback((): void => {
        setHideWithoutWarnings(params, !hideWithoutWarningsValue);
        setParams(params);
    }, [hideWithoutWarningsValue, params, setParams]);

    const onMinorsFilterClick = useCallback((): void => {
        setHideMinors(params, !hideMinorsValue);
        setParams(params);
    }, [hideMinorsValue, params, setParams]);

    const renderedWarnings =
        allWarnings.length === 0 || !props.showWarningFilter ? (
            ''
        ) : (
            <div className="service-lane__warning mdc-top-app-bar__section top-app-bar--narrow-filter">
                {ShownAppsWithWarnings.length} warnings shown ({allWarnings.length} total).
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
                <TeamsFilterDropdown
                    className={'top-app-bar-search-field'}
                    placeholder={'Teams'}
                    leadingIcon={'search'}
                />
            </div>
        ) : (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter"></div>
        );
    const renderedWarningsFilter =
        props.showWarningFilter === true ? (
            <FiltersDropdown
                hideWithoutWarningsValue={hideWithoutWarningsValue}
                hideMinorsValue={hideMinorsValue}
                onWarningsFilterClick={onWarningsFilterClick}
                onMinorsFilterClick={onMinorsFilterClick}></FiltersDropdown>
        ) : (
            <div className="mdc-top-app-bar__section top-app-bar--narrow-filter"></div>
        );
    const renderedUser = cookieValue ? (
        <div className="mdc-top-app-bar__section mdc-top-app-bar__section--wide-filter">
            <span className="welcome-message">
                Welcome, <strong>{loggedInUser}!</strong>
            </span>
        </div>
    ) : (
        <div></div>
    );

    const renderedGeneralGitSyncStatus = props.showGitSyncStatus ? (
        <GeneralGitSyncStatus enabled={true}></GeneralGitSyncStatus>
    ) : (
        <div className="mdc-top-app-bar__section top-app-bar--narrow-filter"></div>
    );
    return (
        <div className="mdc-top-app-bar">
            <div className="top-app-bar__sidebarrow">
                <div className="top-app-bar__mainsection">
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
                        {renderedUser}
                        {renderedGeneralGitSyncStatus}
                    </div>
                </div>
                <div className="top-app-bar__sidebarsection">
                    <SideBar
                        className={classNames(
                            `mdc-drawer-sidebar mdc-drawer-sidebar-container`,
                            'mdc-drawer-sidebar--displayed'
                        )}
                    />
                </div>
            </div>
        </div>
    );
};
