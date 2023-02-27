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

const getRelativeDate = (date: Date): string => {
    const millisecondsPerHour = 1000 * 60 * 60; // 1000ms * 60s * 60m
    const elapsedTime = Date.now().valueOf() - date.valueOf();
    const hoursSinceDate = Math.floor(elapsedTime / millisecondsPerHour);

    if (hoursSinceDate < 24) {
        // recent date, calculate relative time in hours
        if (hoursSinceDate === 0) {
            return '< 1 hour ago';
        } else if (hoursSinceDate === 1) {
            return '1 hour ago';
        } else {
            return `${hoursSinceDate} hours ago`;
        }
    } else {
        // too many hours, calculate relative time in days
        const daysSinceDate = Math.floor(hoursSinceDate / 24);
        if (daysSinceDate === 1) {
            return '1 day ago';
        } else {
            return `${daysSinceDate} days ago`;
        }
    }
};

export const getFormattedReleaseDate = (createdAt: Date): JSX.Element => {
    // Adds leading zero to get two digit day and month
    const twoDigit = (num: number): string => (num < 10 ? '0' : '') + num;
    // date format (ISO): yyyy-mm-dd, with no leading zeros, month is 0-indexed.
    // createdAt.toISOString() can't be used because it ignores the current time zone.
    const formattedDate = `${createdAt.getFullYear()}-${twoDigit(createdAt.getMonth() + 1)}-${twoDigit(
        createdAt.getDate()
    )}`;

    // getHours automatically gets the hours in the correct timezone. in 24h format (no timezone calculation needed)
    const formattedTime = `${createdAt.getHours()}:${createdAt.getMinutes()}`;

    return (
        <div>
            {formattedDate + ' @ ' + formattedTime + ' | '}
            <i>{getRelativeDate(createdAt)}</i>
        </div>
    );
};

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

    const tooltipContents = (
        <h2 className="mdc-tooltip__title release__details">
            {!!sourceMessage && <b>{sourceMessage}</b>}
            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
            {!!sourceAuthor && <div>{'| ' + sourceAuthor + ' |'}</div>}
            {!!createdAt && <div className="release__metadata">{getFormattedReleaseDate(createdAt)}</div>}
        </h2>
    );

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
                        onClick={clickHandler}>
                        <div className="release-card__header">
                            <div className="release__title">{undeployVersion ? 'Undeploy Version' : sourceMessage}</div>
                            {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
                        </div>
                        <div className="mdc-card__ripple" />
                    </div>
                </div>
            </div>
        </Tooltip>
    );
};
