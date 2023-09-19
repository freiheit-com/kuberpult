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

import React from 'react';
import { useArgoCdBaseUrl, useSourceRepoUrl, useBranch, useManifestRepoUrl } from './store';

export const deriveArgoAppLink = (baseUrl: string | undefined, app: string): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return baseUrlSlash + 'applications?search=-' + app;
    }
    return '';
};

export const deriveArgoAppEnvLink = (baseUrl: string | undefined, app: string, env: string): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return baseUrlSlash + 'applications?search=' + env + '-' + app;
    }
    return '';
};

export const deriveArgoTeamLink = (baseUrl: string | undefined, team: string): string | undefined => {
    if (baseUrl) {
        const baseUrlSlash = baseUrl.endsWith('/') ? baseUrl : baseUrl + '/';
        return baseUrlSlash + 'applications?&labels=' + encodeURIComponent('com.freiheit.kuberpult/team=') + team;
    }
    return '';
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
    return '';
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

export const ArgoAppEnvLink: React.FC<{ app: string; env: string }> = (props): JSX.Element => {
    const { app, env } = props;
    const argoBaseUrl = useArgoCdBaseUrl();
    if (!argoBaseUrl) {
        // just render as text, because we do not have a base url:
        return <span>{env}</span>;
    }
    return (
        <a title={'Opens the app in ArgoCd for this environment'} href={deriveArgoAppEnvLink(argoBaseUrl, app, env)}>
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
    if (manifestRepo) {
        return (
            <a
                title={'Opens the release directory in the manifest repository for this release'}
                href={deriveReleaseDirLink(manifestRepo, branch, app, version)}>
                {displayString}
            </a>
        );
    }
    return null;
};
