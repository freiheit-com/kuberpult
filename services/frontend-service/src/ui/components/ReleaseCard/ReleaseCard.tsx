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
import React, { useEffect, useRef } from 'react';
import { MDCRipple } from '@material/ripple';
import { updateReleaseDialog, useRelease } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';

export type ReleaseCardProps = {
    className?: string;
    version: number;
    app: string;
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLDivElement>(null);
    const { className, app, version } = props;
    const { createdAt, sourceMessage, sourceCommitId, sourceAuthor } = useRelease(app, version);
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
        <div className={classNames('mdc-card release-card', className)} onClick={clickHandler}>
            <div className="release-card__header">
                <div className="release__title mdc-typography--headline6">{sourceMessage}</div>
                {!!sourceCommitId && <Button className="release__hash" label={sourceCommitId} />}
            </div>
            <div className="mdc-card__primary-action release-card__description" ref={control} tabIndex={0}>
                <div className="mdc-card__ripple" />
                <div className="release__details">
                    {!!createdAt && (
                        <div className="release__metadata mdc-typography--subtitle2">
                            <div>{'Created at: ' + createdAt.toLocaleDateString()}</div>
                            <div>{'Time ' + createdAt.toLocaleTimeString()}</div>
                        </div>
                    )}
                    <div className="release__version mdc-typography--body2">{'Version: ' + version}</div>
                    <div className="release__author mdc-typography--body1">{'Author: ' + sourceAuthor}</div>
                </div>
                <div className="release__environments">
                    <EnvironmentGroupChipList app={props.app} version={props.version} />
                </div>
            </div>
        </div>
    );
};
