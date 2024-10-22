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
import * as React from 'react';
import classNames from 'classnames';
import { Tooltip } from '../tooltip/tooltip';
import { Locks } from '../../../images';
import { Button } from '../button';
import {
    addAction,
    DisplayApplicationLock,
    DisplayLock,
    getPriorityClassName,
    useArgoCDNamespace,
} from '../../utils/store';
import { DisplayLockRenderer } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';
import { ArgoAppEnvLink } from '../../utils/Links';

export const ApplicationLockDisplay: React.FC<{ lock: DisplayLock }> = (props) => {
    const { lock } = props;
    const deleteLock = React.useCallback(() => {
        addAction({
            action: {
                $case: 'deleteEnvironmentApplicationLock',
                deleteEnvironmentApplicationLock: {
                    environment: lock.environment,
                    lockId: lock.lockId,
                    application: lock.application ?? '',
                },
            },
        });
    }, [lock]);
    const content = <DisplayLockRenderer lock={lock} />;
    const lockIcon = <Locks className="application-lock-icon" />;
    return (
        <Tooltip tooltipContent={content} id={'env-group-chip-id-' + lock.lockId}>
            <div>
                <Button icon={lockIcon} onClick={deleteLock} className={'button-lock'} highlightEffect={false} />
            </div>
        </Tooltip>
    );
};

export const ApplicationLockChip = (props: DisplayApplicationLock): JSX.Element => {
    const { environment, environmentGroup, application, lock } = props;
    const priorityClassName = getPriorityClassName(environmentGroup);
    const name = environment.name;

    const namespace = useArgoCDNamespace();

    const locks = (
        <div className={classNames('app-locks')}>
            <ApplicationLockDisplay key={lock.lockId} lock={lock} />
        </div>
    );
    return (
        <div className={classNames('mdc-evolution-chip', 'application-lock-chip', priorityClassName)} role="row">
            <span
                className="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary"
                role="gridcell">
                <span className="mdc-evolution-chip__text-name">
                    <ArgoAppEnvLink app={application} env={name} namespace={namespace} />
                </span>{' '}
                {locks}
            </span>
        </div>
    );
};
