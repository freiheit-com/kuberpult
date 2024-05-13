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

/**
 * Two buttons combined into one.
 * Inspired by GitHubs merge button.
 * Displays one normal button on the left, and one arrow on the right to select a different option.
 */
export type ExpandButtonProps = {
    onClickSubmit: (shouldLockToo: boolean) => void;
    defaultButtonLabel: string;
};

export const ExpandButton = (props: ExpandButtonProps): JSX.Element => {
    const { onClickSubmit } = props;

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

    return (
        <div className={'expand-button'}>
            <div className={'first-two'}>
                {/* the main button: */}
                <Button
                    onClick={onClickSubmitMain}
                    className={'button-main env-card-deploy-btn mdc-button--unelevated'}
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
                                className={'button-popup env-card-deploy-btn mdc-button--unelevated'}
                                key={'button-second-key'}
                                label={'Deploy only'}
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
