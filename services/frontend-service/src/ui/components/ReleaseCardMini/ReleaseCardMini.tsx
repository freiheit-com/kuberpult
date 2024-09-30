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
import classNames from 'classnames';
import React from 'react';
import { useOpenReleaseDialog, useReleaseOrLog } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { undeployTooltipExplanation } from '../ReleaseDialog/ReleaseDialog';
import { FormattedDate } from '../FormattedDate/FormattedDate';
import { ReleaseVersionWithLinks } from '../ReleaseVersion/ReleaseVersion';

export type ReleaseCardMiniProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCardMini: React.FC<ReleaseCardMiniProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCardMini only displays actual releases, so we can assume that it exists here:
    const firstRelease = useReleaseOrLog(app, version);
    const openReleaseDialog = useOpenReleaseDialog(app, version);
    const release = useReleaseOrLog(app, version);
    if (!firstRelease) {
        return null;
    }
    if (!release) {
        return null;
    }
    const { createdAt, sourceMessage, sourceAuthor, undeployVersion, isMinor } = firstRelease;
    const displayedMessage = undeployVersion ? 'Undeploy Version' : sourceMessage + (isMinor ? '💤' : '');
    const displayedTitle = undeployVersion ? undeployTooltipExplanation : '';
    return (
        <div className={classNames('release-card-mini', className)} onClick={openReleaseDialog}>
            <div className={classNames('release__details-mini', className)}>
                <div className="release__details-header" title={displayedTitle}>
                    <div className="release__details-header-title">{displayedMessage}</div>
                    <div className="release__environments-mini">
                        <EnvironmentGroupChipList app={props.app} version={props.version} />
                    </div>
                </div>
                <div className={'release__details-source-line'}>
                    <ReleaseVersionWithLinks application={app} release={release} />
                </div>
                <div className="release__details-msg">
                    {sourceAuthor + ' | '}
                    {!!createdAt && <FormattedDate createdAt={createdAt} className="release__metadata-mini" />}
                </div>
            </div>
        </div>
    );
};
