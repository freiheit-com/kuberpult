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
import { Button } from '../button';
import { Tooltip } from '../tooltip/tooltip';
import React, { useEffect } from 'react';
import { useOpenReleaseDialog, useReleaseOrThrow } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const { className, app, version } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const { createdAt, sourceMessage, sourceCommitId, sourceAuthor, undeployVersion } = useReleaseOrThrow(app, version);
    const openReleaseDialog = useOpenReleaseDialog(app, version);

    useEffect(() => {}, []);

    const tooltipContents = (
        <div className="mdc-tooltip__title_ release__details">
            {!!sourceMessage && <b>{sourceMessage}</b>}
            {!!sourceCommitId && (
                <div className={'release__hash--container'}>
                    <Button className="release__hash" label={'' + sourceCommitId} />
                </div>
            )}
            {!!sourceAuthor && (
                <div>
                    <span>Author:</span> {sourceAuthor}
                </div>
            )}
            {!!createdAt && (
                <div className="release__metadata">
                    <span>Created </span>
                    <FormattedDate className={'date'} createdAt={createdAt} />
                </div>
            )}
        </div>
    );
    const firstLine = sourceMessage.split('\n')[0];
    return (
        <Tooltip id={app + version} tooltipContent={tooltipContents}>
            <div className="release-card__container">
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} smallEnvChip />
                </div>
                <div className={classNames('mdc-card release-card', className)}>
                    <div
                        className="mdc-card__primary-action release-card__description"
                        // ref={control}
                        tabIndex={0}
                        onClick={openReleaseDialog}>
                        <div className="release-card__header">
                            <div className="release__title">{undeployVersion ? 'Undeploy Version' : firstLine}</div>
                            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
                        </div>
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </div>
        </Tooltip>
    );
};
