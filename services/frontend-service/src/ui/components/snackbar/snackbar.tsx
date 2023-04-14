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
import { useEffect, useRef } from 'react';
import { MDCSnackbar } from '@material/snackbar';
import classNames from 'classnames';
import { Button } from '../button';
import { Close } from '../../../images';
import { SnackbarStatus, UpdateSnackbar, useSnackbar } from '../../utils/store';

export const Snackbar = (): JSX.Element => {
    const MDComponent = useRef<MDCSnackbar>();
    const control = useRef<HTMLElement>(null);
    const [show, status, content] = useSnackbar(({ show, status, content }) => [show, status, content]);

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCSnackbar(control.current);
        }
        return (): void => MDComponent.current?.destroy();
    }, []);

    useEffect(() => {
        if (show) {
            // open the snackbar and then set show to false. the snackbar will remain opened 5s
            if (status === SnackbarStatus.WARN) {
                // Warn is used for connection errors
                // when you can't connect, always show a warning
                // eslint-disable-next-line no-type-assertion/no-type-assertion
                MDComponent.current!.timeoutMs = -1;
            } else {
                // snackbar closes after 5s
                // eslint-disable-next-line no-type-assertion/no-type-assertion
                MDComponent.current!.timeoutMs = 5000;
            }
            MDComponent.current?.open();
            UpdateSnackbar.set({ show: false });
        }
    }, [show, status]);

    return (
        <aside
            className={classNames(
                'mdc-snackbar',
                'mdc-snackbar--leading',
                status === SnackbarStatus.SUCCESS && 'mdc-snackbar--success',
                status === SnackbarStatus.WARN && 'mdc-snackbar--warn',
                status === SnackbarStatus.ERROR && 'mdc-snackbar--error'
            )}
            ref={control}>
            <div className="mdc-snackbar__surface" role="status" aria-relevant="additions">
                <div className="mdc-snackbar__label" aria-atomic="false">
                    <b>{content}</b>
                </div>
                <div className="mdc-snackbar__actions" aria-atomic="true">
                    <Button icon={<Close width="18px" height="18px" />} className={'mdc-snackbar__action'} />
                </div>
            </div>
        </aside>
    );
};
