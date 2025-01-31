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
import { useCallback, useState } from 'react';
import * as React from 'react';
import { Button } from './button';
import { PlainDialog } from '../dialog/ConfirmationDialog';
import classNames from 'classnames';

/**
 * Two buttons combined into one.
 * Inspired by GitHubs merge button.
 * Displays one normal button on the left, and one arrow on the right to select a different option.
 */

export type ExpandButtonProps = {
    onClickSubmit: (shouldLockToo: boolean) => void;
    onClickLock: () => void;
    defaultButtonLabel: string;
    disabled: boolean;
    releaseDifference: number;
    alreadyPlanned: boolean;
};

export const ExpandButton = (props: ExpandButtonProps): JSX.Element => {
    const { onClickSubmit, onClickLock, releaseDifference, alreadyPlanned } = props;

    const [expanded, setExpanded] = useState(false);

    const onClickExpand = useCallback(() => {
        setExpanded(!expanded);
    }, [setExpanded, expanded]);

    const onClickClose = useCallback(() => {
        setExpanded(false);
    }, [setExpanded]);

    const onClickSubmitMain = useCallback(() => {
        onClickSubmit(true);
    }, [onClickSubmit]);

    const onClickSubmitAlternative = useCallback(() => {
        onClickSubmit(false);
    }, [onClickSubmit]);

    const onClickSubmitLockOnly = useCallback(() => {
        onClickLock();
    }, [onClickLock]);

    const deployLabel =
        releaseDifference < 0 ? 'Update only' : releaseDifference === 0 ? 'Deploy only' : 'Rollback only';

    return (
        <div className={'expand-button'}>
            <div className={'first-two'}>
                {/* the main button: */}
                <Button
                    onClick={onClickSubmitMain}
                    disabled={props.disabled}
                    className={classNames('button-main', 'env-card-deploy-btn', 'mdc-button--unelevated', {
                        'deploy-button-cancel': alreadyPlanned,
                    })}
                    key={'button-first-key'}
                    label={props.defaultButtonLabel}
                    highlightEffect={false}
                />
                {/* the button to expand the dialog: */}
                <Button
                    onClick={onClickExpand}
                    className={'button-expand'}
                    key={'button-second-key'}
                    label={''}
                    icon={<div className={'dropdown-arrow'}>⌄</div>}
                    highlightEffect={false}
                />
            </div>
            {expanded && (
                <PlainDialog
                    open={expanded}
                    onClose={onClickClose}
                    classNames={'expand-dialog'}
                    disableBackground={false}
                    center={false}>
                    <>
                        <div>
                            <Button
                                onClick={onClickSubmitAlternative}
                                className={classNames(
                                    'button-popup-deploy',
                                    'env-card-deploy-btn',
                                    'mdc-button--unelevated',
                                    { 'deploy-button-cancel': alreadyPlanned }
                                )}
                                key={'button-second-key'}
                                label={alreadyPlanned ? `Cancel ${deployLabel}` : deployLabel}
                                icon={undefined}
                                highlightEffect={true}
                                disabled={props.disabled}
                            />
                        </div>
                        <div>
                            <Button
                                onClick={onClickSubmitLockOnly}
                                className={'button-popup-lock env-card-lock-btn mdc-button--unelevated'}
                                key={'button-third-key'}
                                label={'Lock only'}
                                icon={undefined}
                                highlightEffect={true}
                            />
                        </div>
                    </>
                </PlainDialog>
            )}
        </div>
    );
};
