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
import { updateReleaseDialog, useRelease } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';
import { daysToString } from '../LockDisplay/LockDisplay';

const MsPerDay = 1000 * 60 * 60 * 24;

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLDivElement>(null);
    const { className, app, version } = props;
    const { createdAt, sourceMessage, sourceCommitId, sourceAuthor, undeployVersion } = useRelease(app, version);
    const clickHandler = React.useCallback(() => {
        updateReleaseDialog(app, version);
    }, [app, version]);

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return (): void => MDComponent.current?.destroy();
    }, []);

    return (
        <Tooltip
            id={app + version}
            content={
                <>
                    <h2 className="mdc-tooltip__title release__details">
                        {!!sourceMessage && <b>{sourceMessage}</b>}
                        {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
                        {!!sourceAuthor && <div>{'| ' + sourceAuthor + ' |'}</div>}
                        {!!createdAt && (
                            <div className="release__metadata mdc-typography--subtitle2">
                                <div>
                                    {`${createdAt.getDay()}-${createdAt.getMonth()}-${createdAt.getFullYear()}` +
                                        ' @ ' +
                                        `${createdAt.getHours()}:${createdAt.getMinutes()}` +
                                        ' | '}
                                    <i>
                                        {daysToString(((Date.now().valueOf() - createdAt.valueOf()) / MsPerDay) >> 0)}
                                    </i>
                                </div>
                            </div>
                        )}
                    </h2>
                </>
            }>
            <>
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} useFirstLetter />
                </div>
                <div className={classNames('mdc-card release-card', className)}>
                    <div
                        className="mdc-card__primary-action release-card__description"
                        ref={control}
                        tabIndex={0}
                        onClick={clickHandler}>
                        <div className="release-card__header">
                            <div className="release__title mdc-typography--headline6">
                                {undeployVersion ? 'Undeploy Version' : sourceMessage}
                            </div>
                            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
                        </div>
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </>
        </Tooltip>
    );
};
