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
import { useCallback } from 'react';
import * as React from 'react';
import { Button } from './button';
import classNames from 'classnames';

/**
 * Two buttons combined into one.
 * Inspired by GitHubs merge button.
 * Displays one normal button on the left, and one arrow on the right to select a different option.
 */

export type DeployLockButtonsProps = {
    onClickSubmit: (shouldLockToo: boolean) => void;
    onClickLock: () => void;
    disabled: boolean;
    releaseDifference: number;
    deployAlreadyPlanned: boolean;
    lockAlreadyPlanned: boolean;
    hasLocks: boolean;
    unlockAlreadyPlanned: boolean;
};

export const DeployLockButtons = (props: DeployLockButtonsProps): JSX.Element => {
    const {
        onClickSubmit,
        onClickLock,
        releaseDifference,
        deployAlreadyPlanned,
        lockAlreadyPlanned,
        hasLocks,
        unlockAlreadyPlanned,
    } = props;

    const onClickDeploy = useCallback(() => {
        onClickSubmit(!deployAlreadyPlanned && !lockAlreadyPlanned);
    }, [onClickSubmit, deployAlreadyPlanned, lockAlreadyPlanned]);

    const deployType = releaseDifference < 0 ? 'Update' : releaseDifference === 0 ? 'Deploy' : 'Rollback';
    const deployOnly = lockAlreadyPlanned ? '' : ' and Lock';
    const deployLabel = deployAlreadyPlanned ? `Cancel ${deployType}` : `${deployType}${deployOnly}`;

    const lockOnly = deployAlreadyPlanned ? '' : ' Only';
    const lockLabel = lockAlreadyPlanned
        ? 'Cancel Planned Lock'
        : !hasLocks
          ? `Add Lock${lockOnly}`
          : unlockAlreadyPlanned
            ? 'Keep Locks'
            : 'Remove Locks';

    return (
        <div className="deploy-lock-buttons">
            <Button
                onClick={onClickLock}
                className={classNames('button-popup-lock', 'env-card-lock-btn', 'mdc-button--unelevated', {
                    'deploy-button-cancel': lockAlreadyPlanned || unlockAlreadyPlanned,
                })}
                key={'button-third-key'}
                label={lockLabel}
                icon={undefined}
                highlightEffect={true}
            />
            <Button
                onClick={onClickDeploy}
                disabled={props.disabled}
                className={classNames('button-main', 'env-card-deploy-btn', 'mdc-button--unelevated', {
                    'deploy-button-cancel': deployAlreadyPlanned,
                })}
                key={'button-first-key'}
                label={deployLabel}
                highlightEffect={false}
            />
        </div>
    );
};
