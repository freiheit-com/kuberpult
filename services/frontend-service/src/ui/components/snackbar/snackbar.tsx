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
import React, { useCallback, useEffect } from 'react';
import { Button } from '../button';
import { Close } from '../../../images';
import { SnackbarStatus, UpdateSnackbar, useSnackbar } from '../../utils/store';
import { PlainDialog } from '../dialog/ConfirmationDialog';

const showSnackbarDurationMillis: number = 15 * 1000;

export const Snackbar = (): JSX.Element => {
    const [show, status, content] = useSnackbar(({ show, status, content }) => [show, status, content]);
    useEffect(() => {
        if (!show) {
            return;
        }
        const timer1 = setTimeout(() => {
            UpdateSnackbar.set({ show: false });
        }, showSnackbarDurationMillis);

        return () => {
            clearTimeout(timer1);
        };
    }, [show]);

    const cssColor: string =
        status === SnackbarStatus.SUCCESS
            ? 'success'
            : status === SnackbarStatus.WARN
              ? 'warn'
              : status === SnackbarStatus.ERROR
                ? 'error'
                : 'invalid-color';

    const onClickClose = useCallback(() => {
        UpdateSnackbar.set({ show: false });
    }, []);

    return (
        <PlainDialog
            open={show}
            onClose={onClickClose}
            classNames={`k-snackbar snackbar-color-${cssColor}`}
            disableBackground={false}
            center={false}>
            <div className={''}>
                <div className={'k-snackbar-content'}>
                    <div className={'k-snackbar-text'}>
                        <span>{content}</span>
                    </div>
                    <div className={'k-snackbar-button'}>
                        <span>
                            <Button
                                onClick={onClickClose}
                                icon={<Close width="18px" height="18px" />}
                                highlightEffect={false}
                            />
                        </span>
                    </div>
                </div>
            </div>
        </PlainDialog>
    );
};
