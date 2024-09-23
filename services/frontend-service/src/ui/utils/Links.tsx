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

import React from 'react';
import { useArgoCdBaseUrl, useSourceRepoUrl, useBranch, useManifestRepoUrl } from './store';
import classNames from 'classnames';

export const deriveArgoAppLink = (baseUrl: string | undefined, app: string): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return baseUrlSlash + 'applications?search=-' + app;
    }
    return undefined;
};

export const deriveArgoAppEnvLink = (
    baseUrl: string | undefined,
    app: string,
    env: string,
    namespace: string
): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return `${baseUrlSlash}applications/${namespace}/${env}-${app}`;
    }
    return undefined;
};

export const deriveArgoTeamLink = (baseUrl: string | undefined, team: string): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return baseUrlSlash + 'applications?&labels=' + encodeURIComponent('com.freiheit.kuberpult/team=') + team;
    }
    return undefined;
};

export const deriveSourceCommitLink = (
    baseUrl: string | undefined,
    branch: string | undefined,
    commit: string
): string | undefined => {
    if (baseUrl && branch) {
        baseUrl = baseUrl.replace(/{branch}/gi, branch);
        baseUrl = baseUrl.replace(/{commit}/gi, commit);
        return baseUrl;
    }
    return undefined;
};

export const deriveReleaseDirLink = (
    baseUrl: string | undefined,
    branch: string | undefined,
    app: string,
    version: number
): string | undefined => {
    if (baseUrl && branch) {
        baseUrl = baseUrl.replace(/{branch}/gi, branch);
        baseUrl = baseUrl.replace(/{dir}/gi, 'applications/' + app + '/releases/' + version);
        return baseUrl;
    }
    return undefined;
};

export const getCommitHistoryLink = (commitId: string | undefined): string | undefined => {
    if (commitId) {
        return '/ui/commits/' + commitId;
    }
    return undefined;
};

export const ArgoTeamLink: React.FC<{ team: string | undefined }> = (props): JSX.Element | null => {
    const { team } = props;
    const argoBaseUrl = useArgoCdBaseUrl();
    if (!team) {
        return null;
    }
    if (!argoBaseUrl) {
        // just render as text, because we do not have a base url:
        return <span>{team}</span>;
    }
    return (
        <span>
            {' | Team: '}
            <a title={'Opens the team in ArgoCd'} href={deriveArgoTeamLink(argoBaseUrl, team)}>
                {team}
            </a>
        </span>
    );
};

export const ArgoAppLink: React.FC<{ app: string }> = (props): JSX.Element => {
    const { app } = props;
    const argoBaseUrl = useArgoCdBaseUrl();
    if (!argoBaseUrl) {
        // just render as text, because we do not have a base url:
        return <span>{app}</span>;
    }
    return (
        <a title={'Opens this app in ArgoCd for all environments'} href={deriveArgoAppLink(argoBaseUrl, app)}>
            {app}
        </a>
    );
};

export const ArgoAppEnvLink: React.FC<{ app: string; env: string; namespace: string | undefined }> = (
    props
): JSX.Element => {
    const { app, env, namespace } = props;
    const argoBaseUrl = useArgoCdBaseUrl();
    if (!argoBaseUrl) {
        // just render as text, because we do not have a base url:
        return <span>{env}</span>;
    }
    return (
        <a
            title={'Opens the app in ArgoCd for this environment'}
            className={classNames('env-card-link')}
            href={namespace ? deriveArgoAppEnvLink(argoBaseUrl, app, env, namespace) : undefined}>
            {env}
        </a>
    );
};

export const DisplaySourceLink: React.FC<{ displayString: string; commitId: string }> = (props): JSX.Element | null => {
    const { commitId, displayString } = props;
    const sourceRepo = useSourceRepoUrl();
    const branch = useBranch();
    const sourceLink = deriveSourceCommitLink(sourceRepo, branch, commitId);
    if (sourceLink) {
        return (
            <a title={'Opens the commit for this release in the source repository'} href={sourceLink}>
                {displayString}
            </a>
        );
    }
    return null;
};

export const DisplayManifestLink: React.FC<{ displayString: string; app: string; version: number }> = (
    props
): JSX.Element | null => {
    const { displayString, app, version } = props;
    const manifestRepo = useManifestRepoUrl();
    const branch = useBranch();
    const manifestLink = deriveReleaseDirLink(manifestRepo, branch, app, version);
    if (manifestLink && version) {
        return (
            <a title={'Opens the release directory in the manifest repository for this release'} href={manifestLink}>
                {displayString}
            </a>
        );
    }
    return null;
};

export const DisplayCommitHistoryLink: React.FC<{ displayString: string; commitId: string }> = (
    props
): JSX.Element | null => {
    const { displayString, commitId } = props;
    if (commitId) {
        const listLink = getCommitHistoryLink(commitId);
        return (
            <a title={'Opens the commit history'} href={listLink}>
                {displayString}
            </a>
        );
    }

    return null;
};

type Query = {
    key: string;
    value: string | null;
};

const toQueryString = (queries: Query[]): string => {
    const str: string[] = [];
    queries.forEach((q: Query) => {
        if (q.value) {
            str.push(encodeURIComponent(q.key) + '=' + encodeURIComponent(q.value));
        }
    });
    return str.join('&');
};

export const ProductVersionLink: React.FC<{ env: string; groupName: string }> = (props): JSX.Element | null => {
    const { env, groupName } = props;

    const separator = groupName === '' ? '' : '/';

    const urlParams = new URLSearchParams(window.location.search);
    const teams = urlParams.get('teams');
    const queryString = toQueryString([
        { key: 'env', value: groupName + separator + env },
        { key: 'teams', value: teams },
    ]);

    const currentLink = window.location.href;
    const addParam = currentLink.split('?');
    return (
        <a
            title={'Opens the release directory in the manifest repository for this release'}
            href={addParam[0] + '/productVersion?' + queryString}>
            Display Version for {env}
        </a>
    );
};

export const KuberpultGitHubLink: React.FC<{ version: string }> = (props): JSX.Element | null => {
    const { version } = props;
    return (
        <a
            title={'Opens the Kuberpult Readme for the current version ' + version}
            href={'https://github.com/freiheit-com/kuberpult/blob/' + version + '/README.md'}>
            {version}
        </a>
    );
};

const hideWithoutWarningsParamName = 'hideWithoutWarnings';
const hideWithoutWarningsParamEnabledValue = 'Y';
export const hideWithoutWarnings = (params: URLSearchParams): boolean => {
    const hideWithoutWarningsParam = params.get(hideWithoutWarningsParamName) || '';
    return hideWithoutWarningsParam === hideWithoutWarningsParamEnabledValue;
};
export const setHideWithoutWarnings = (params: URLSearchParams, newValue: boolean): void => {
    if (newValue) {
        params.set(hideWithoutWarningsParamName, hideWithoutWarningsParamEnabledValue);
    } else {
        params.delete(hideWithoutWarningsParamName);
    }
};

const hideMinorsParamName = 'hideMinors';
const hideMinorsParamEnabledValue = 'Y';
export const hideMinors = (params: URLSearchParams): boolean => {
    const hideMinorsParam = params.get(hideMinorsParamName) || '';
    return hideMinorsParam === hideMinorsParamEnabledValue;
};
export const setHideMinors = (params: URLSearchParams, newValue: boolean): void => {
    if (newValue) {
        params.set(hideMinorsParamName, hideMinorsParamEnabledValue);
    } else {
        params.delete(hideMinorsParamName);
    }
};

const envConfigDialogParamName = 'dialog-env-config';
export const getOpenEnvironmentConfigDialog = (params: URLSearchParams): string =>
    params.get(envConfigDialogParamName) || '';
export const setOpenEnvironmentConfigDialog = (params: URLSearchParams, envName: string): void => {
    if (envName.length > 0) {
        params.set(envConfigDialogParamName, envName);
    } else {
        params.delete(envConfigDialogParamName);
    }
};

export const ReleaseTrainLink: React.FC<{ env: string }> = (props): JSX.Element | null => {
    const { env } = props;

    const currentLink = window.location.href;

    return (
        <a
            title={'Opens a preview of release trains on this environment'}
            href={currentLink + '/' + env + '/releaseTrain'}>
            Release train details
        </a>
    );
};
