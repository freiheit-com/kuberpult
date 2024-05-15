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
import React, { useCallback, useEffect, useState } from 'react';
import { Button } from '../button';
import { Close } from '../../../images';
import { PlainDialog } from '../dialog/ConfirmationDialog';
import { useSearchParams } from 'react-router-dom';
import { getOpenEnvironmentConfigDialog, setOpenEnvironmentConfigDialog } from '../../utils/Links';
import { Spinner } from '../Spinner/Spinner';
import { GetEnvironmentConfigPretty, showSnackbarError } from '../../utils/store';

export const environmentConfigDialogClass = 'environment-config-dialog';
const environmentConfigDialogAppBarClass = environmentConfigDialogClass + '-app-bar';
const environmentConfigDialogDataClass = environmentConfigDialogClass + '-app-bar-data';
const environmentConfigDialogNameClass = environmentConfigDialogClass + '-name';
export const environmentConfigDialogCloseClass = environmentConfigDialogClass + '-close';
export const environmentConfigDialogConfigClass = environmentConfigDialogClass + '-config';

export type EnvironmentConfigDialogProps = {
    environmentName: string;
};

export const EnvironmentConfigDialog: React.FC<EnvironmentConfigDialogProps> = (props) => {
    const environmentName = props.environmentName;
    const [params, setParams] = useSearchParams();
    const isOpen = (): boolean => getOpenEnvironmentConfigDialog(params) === props.environmentName;
    const onClose = useCallback((): void => {
        setOpenEnvironmentConfigDialog(params, '');
        setParams(params);
    }, [params, setParams]);
    const [config, setConfig] = useState('');
    const [loading, setLoading] = useState(false);
    useEffect(() => {
        if (getOpenEnvironmentConfigDialog(params) !== environmentName) {
            return;
        }
        setLoading(true);
        const result = GetEnvironmentConfigPretty(environmentName);
        result
            .then((pretty) => {
                setLoading(false);
                setConfig(pretty);
            })
            .catch((e) => {
                setLoading(false);
                showSnackbarError('Error loading environment configuration.');
                // eslint-disable-next-line no-console
                console.error('error while loading environment config: ' + e);
                setConfig('');
            });
    }, [environmentName, params]);

    const dialog: JSX.Element | '' = (
        <PlainDialog
            open={isOpen()}
            onClose={onClose}
            classNames={environmentConfigDialogClass}
            disableBackground={true}
            center={true}>
            <>
                <div className={environmentConfigDialogAppBarClass}>
                    <div className={environmentConfigDialogDataClass}>
                        <div className={environmentConfigDialogNameClass}>Environment Config for {environmentName}</div>
                    </div>
                    <Button
                        onClick={onClose}
                        className={environmentConfigDialogCloseClass}
                        icon={<Close />}
                        highlightEffect={false}
                    />
                </div>
                {loading ? (
                    <Spinner message="loading" />
                ) : (
                    <pre className={environmentConfigDialogConfigClass}>{config}</pre>
                )}
            </>
        </PlainDialog>
    );
    return <div>{dialog}</div>;
};
