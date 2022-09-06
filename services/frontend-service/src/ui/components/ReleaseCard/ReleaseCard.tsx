/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import classNames from 'classnames';
import { Button } from '../button';
import { useEffect, useRef } from 'react';
import { MDCRipple } from '@material/ripple';
import { Chip } from '../chip';

export type ReleaseCardProps = {
    className?: string;
    title: string;
    hash?: string;
    createdAt: Date;
    author: string;
    environments: string[];
};

export const ReleaseCard: React.FC<ReleaseCardProps> = (props) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLDivElement>(null);
    const { className, title, hash, createdAt, author, environments } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <div className={classNames('mdc-card release-card', className)}>
            <div className="release-card__header">
                <div className="release__title mdc-typography--headline6">{title}</div>
                {!!hash && <Button className="release__hash" label={hash} />}
            </div>
            <div className="mdc-card__primary-action release-card__description" ref={control} tabIndex={0}>
                <div className="mdc-card__ripple"></div>
                <div className="release__details">
                    <div className="release__metadata mdc-typography--subtitle2">
                        <div>{'Created at: ' + createdAt.toLocaleDateString()}</div>
                        <div>{'Time ' + createdAt.toLocaleTimeString()}</div>
                    </div>
                    <div className="release__author mdc-typography--body1">{'Author: ' + author}</div>
                </div>
                <div className="release__environments">
                    {environments.map((env) => (
                        <Chip className={'release-environment release-environment--' + env} label={env} key={env} />
                    ))}
                </div>
            </div>
        </div>
    );
};
