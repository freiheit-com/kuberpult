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

/**
 * Two buttons combined into one.
 * Inspired by GitHubs merge button.
 * Displays one normal button on the left, and one arrow on the right to select a different option.
 */
export const ExpandButton = (props: {
    onClickSubmit: (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => void;
}): JSX.Element => {
    const { onClickSubmit } = props;

    const [expanded, setExpanded] = useState(false);

    const onClickExpand = useCallback(() => {
        setExpanded(!expanded);
    }, [setExpanded, expanded]);

    // const  = useCallback(() => {
    //     setShouldLockToo(!shouldLockToo);
    // }, [shouldLockToo, setShouldLockToo]);

    return (
        <div>
            <Button onClick={onClickSubmit} className={'button-first'} key={'button-first-key'} />
            <Button onClick={onClickExpand} className={'button-second'} key={'button-second-key'} />

            {expanded ? 'expanded' : 'not-expanded'}
        </div>
    );
};
