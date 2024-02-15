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
import React, { useCallback, useEffect, useState } from 'react';
import { Button } from '../button';
import { Close } from '../../../images';
import { PlainDialog } from '../dialog/ConfirmationDialog';
import { useSearchParams } from 'react-router-dom';
import { getOpenEnvironmentConfigDialog, setOpenEnvironmentConfigDialog } from '../../utils/Links';
import { useApi } from '../../utils/GrpcApi';
import { Spinner } from '../Spinner/Spinner';

export type EnvironmentConfigDialogProps = {
    environmentName: string;
};

export const EnvironmentConfigDialog: React.FC<EnvironmentConfigDialogProps> = (props) => {
    const environmentName = props.environmentName;
    const [params, setParams] = useSearchParams();
    const api = useApi;
    const isOpen = useCallback((): boolean => getOpenEnvironmentConfigDialog(params).length > 0, [params]);
    const onClose = useCallback((): void => {
        setOpenEnvironmentConfigDialog(params, '');
        setParams(params);
    }, [params, setParams]);
    const [config, setConfig] = useState('');
    const [loading, setLoading] = useState(true);
    useEffect(() => {
        if (getOpenEnvironmentConfigDialog(params) !== environmentName) {
            setLoading(true);
            setConfig(''); // we are invisible and show a spinner, so empty until that changes.
            return;
        }
        const result = api.environmentService().GetEnvironmentConfig({ environment: environmentName });
        result.then((res) => {
            const pretty = JSON.stringify(res, null, ' ');
            setConfig(pretty);
            setLoading(false);
        });
        result.catch((e) => {
            // eslint-disable-next-line no-console
            console.error('error while loading environment config: ' + e);
        });
    }, [api, environmentName, params]);

    const dialog: JSX.Element | '' = (
        <PlainDialog
            open={isOpen()}
            onClose={onClose}
            classNames={'environment-config-dialog'}
            disableBackground={true}
            center={true}>
            <>
                <div className={'environment-config-dialog-app-bar'}>
                    <div className={'environment-config-dialog-app-bar-data'}>
                        <div className={'environment-config-dialog-name'}>Environment Config for {environmentName}</div>
                    </div>
                    <Button onClick={onClose} className={'environment-config-dialog-close'} icon={<Close />} />
                </div>
                {loading ? <Spinner message="loading ..." /> : <pre>{config}</pre>}
            </>
        </PlainDialog>
    );
    return <div>{dialog}</div>;
};
