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
import { useCallback, useState } from 'react';
import * as React from 'react';
import { Button } from './button';
import { PlainDialog } from '../dialog/ConfirmationDialog';

/**
 * Two buttons combined into one.
 * Inspired by GitHubs merge button.
 * Displays one normal button on the left, and one arrow on the right to select a different option.
 */
export const ExpandButton = (props: {
    onClickSubmit: (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => void;
    defaultButtonLabel: string;
    defaultButtonIcon: JSX.Element;
}): JSX.Element => {
    const { onClickSubmit } = props;

    const [expanded, setExpanded] = useState(false);

    const onClickExpand = useCallback(() => {
        // eslint-disable-next-line no-console
        console.info('SU DEBUG: click old: expanded=', expanded); // eslint-disable-next-line no-console
        setExpanded(!expanded);
    }, [setExpanded, expanded]);
    const onClickClose = useCallback(() => {
        setExpanded(false);
    }, [setExpanded]);

    // const  = useCallback(() => {
    //     setShouldLockToo(!shouldLockToo);
    // }, [shouldLockToo, setShouldLockToo]);

    return (
        <div className={'expand-button'}>
            {/* the main button: */}
            <Button
                id={'expand-1'}
                onClick={onClickSubmit}
                className={'button-first env-card-deploy-btn mdc-button--unelevated'}
                key={'button-first-key'}
                label={props.defaultButtonLabel}
            />
            {/* the button to expand the dialog: */}
            <Button
                id={'expand-2'}
                onClick={onClickExpand}
                className={'button-second'}
                key={'button-second-key'}
                label={''}
                icon={<div className={'dropdown-arrow'}>⌄</div>}
            />
            {/*TODO SU REMOVE: */}
            {/*{expanded ? 'exp' : 'not-exp'}*/}
            {expanded && (
                <PlainDialog
                    open={expanded}
                    onClose={onClickClose}
                    classNames={'expand-dialog'}
                    disableBackground={false}
                    center={false}>
                    <div>
                        {/*<div>Deploy without creating a lock:</div>*/}
                        <Button
                            id={'expand-3'}
                            onClick={onClickExpand}
                            className={'button-second env-card-deploy-btn mdc-button--unelevated'}
                            key={'button-second-key'}
                            label={'Deploy only'}
                            icon={undefined}
                        />
                    </div>
                </PlainDialog>
            )}
        </div>
    );
};
