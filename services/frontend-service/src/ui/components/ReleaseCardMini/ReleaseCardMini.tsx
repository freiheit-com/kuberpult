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
import classNames from 'classnames';
import React from 'react';
import { useOpenReleaseDialog, useReleaseOrThrow } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { undeployTooltipExplanation } from '../ReleaseDialog/ReleaseDialog';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { ReleaseVersionLink } from '../../utils/Links';

export type ReleaseCardMiniProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCardMini: React.FC<ReleaseCardMiniProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCardMini only displays actual releases, so we can assume that it exists here:
    const { createdAt, sourceMessage, sourceAuthor, undeployVersion } = useReleaseOrThrow(app, version);
    const openReleaseDialog = useOpenReleaseDialog(app, version);
    const displayedMessage = undeployVersion ? 'Undeploy Version' : sourceMessage;
    const displayedTitle = undeployVersion ? undeployTooltipExplanation : '';
    const release = useReleaseOrThrow(app, version);
    return (
        <div className={classNames('release-card-mini', className)} onClick={openReleaseDialog}>
            <div className={classNames('release__details-mini', className)}>
                <div className="release__details-header" title={displayedTitle}>
                    {displayedMessage}
                </div>
                <ReleaseVersionLink
                    displayVersion={release.displayVersion}
                    undeployVersion={undeployVersion}
                    sourceCommitId={''}
                    version={version}
                    app={app}
                />
                <div className="release__details-msg">
                    {sourceAuthor + ' | '}
                    {!!createdAt && <FormattedDate createdAt={createdAt} className="release__metadata-mini" />}
                </div>
            </div>
            <div className="release__environments-mini">
                <EnvironmentGroupChipList app={props.app} version={props.version} />
            </div>
        </div>
    );
};
