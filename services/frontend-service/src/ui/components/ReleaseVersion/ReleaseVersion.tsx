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
import { Release } from '../../../api/api';
import { DisplayManifestLink, DisplaySourceLink } from '../../utils/Links';

export type ReleaseVersionProps = {
    release: Pick<Release, 'version' | 'sourceCommitId' | 'displayVersion' | 'undeployVersion'>;
};

export type ReleaseVersionWithLinksProps = {
    application: string;
    release: Pick<Release, 'version' | 'sourceCommitId' | 'displayVersion' | 'undeployVersion'>;
};

export const ReleaseVersion: React.FC<ReleaseVersionProps> = ({ release }) => {
    if (release.undeployVersion) {
        return (
            <span className="release-version__undeploy-version" title="Remove">
                undeploy
            </span>
        );
    } else if (release.displayVersion !== '') {
        return (
            <span className="release-version__display-version" title={release.sourceCommitId}>
                {release.displayVersion}
            </span>
        );
    } else if (release.sourceCommitId !== '') {
        return (
            <span className="release-version__commit-id" title={release.sourceCommitId}>
                {release.sourceCommitId.substring(0, 8)}
            </span>
        );
    } else {
        return <span className="release-version__version">#{release.version}</span>;
    }
};

export const ReleaseVersionWithLinks: React.FC<ReleaseVersionWithLinksProps> = ({ release, application }) => (
    <div className={'links'}>
        <div className={'links-left'}>
            <ReleaseVersion release={release} />{' '}
        </div>
        <div className={'links-right'}>
            <DisplaySourceLink displayString={'Source'} commitId={release.sourceCommitId} />{' '}
            <DisplayManifestLink version={release.version} app={application} displayString={'Manifest'} />
        </div>
    </div>
);
