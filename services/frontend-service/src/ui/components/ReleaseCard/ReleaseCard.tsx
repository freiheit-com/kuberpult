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
import React, { useEffect, useRef } from 'react';
import { MDCRipple } from '@material/ripple';
import { useOpenReleaseDialog, useReleaseOrThrow } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { FormattedDate } from '../FormattedDate/FormattedDate';

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLDivElement>(null);
    const { className, app, version } = props;
    // the ReleaseCard only displays actual releases, so we can assume that it exists here:
    const { createdAt, sourceMessage, sourceCommitId, sourceAuthor, undeployVersion } = useReleaseOrThrow(app, version);
    const openReleaseDialog = useOpenReleaseDialog(app, version);

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return (): void => MDComponent.current?.destroy();
    }, []);

    const tooltipContents = (
        <h2 className="mdc-tooltip__title release__details">
            {!!sourceMessage && <b>{sourceMessage}</b>}
            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
            {!!sourceAuthor && <div>{'| ' + sourceAuthor + ' |'}</div>}
            {!!createdAt && <FormattedDate createdAt={createdAt} className="release__metadata" />}
        </h2>
    );
    const firstLine = sourceMessage.split('\n')[0];
    return (
        <Tooltip id={app + version} content={tooltipContents}>
            <div className="release-card__container">
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} smallEnvChip />
                </div>
                <div className={classNames('mdc-card release-card', className)}>
                    <div
                        className="mdc-card__primary-action release-card__description"
                        ref={control}
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
