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
import * as React from 'react';
import { BrowserHeaders } from 'browser-headers';
import { Configuration, PublicClientApplication } from '@azure/msal-browser';
import {
    MsalProvider,
    AuthenticatedTemplate,
    useIsAuthenticated,
    UnauthenticatedTemplate,
    useMsal,
} from '@azure/msal-react';
import { GetFrontendConfigResponse } from '../../api/api';
import { createStore } from 'react-use-sub';
import { grpc } from '@improbable-eng/grpc-web';

type FrontendConfig = {
    configs: GetFrontendConfigResponse;
    configReady: boolean;
};

export const [useFrontendConfig, UpdateFrontendConfig] = createStore<FrontendConfig>({
    configs: {
        sourceRepoUrl: '',
        kuberpultVersion: '0',
    },
    configReady: false,
});

type AzureAuthSubType = {
    authHeader: grpc.Metadata & {
        Authorization?: String;
    };
    authReady: boolean;
};

export const [useAzureAuthSub, AzureAuthSub] = createStore<AzureAuthSubType>({
    authHeader: new BrowserHeaders({}),
    authReady: false,
});

const getMsalConfig = (configs: GetFrontendConfigResponse): Configuration => ({
    auth: {
        clientId: configs.authConfig?.azureAuth?.clientId || '',
        authority: `${configs.authConfig?.azureAuth?.cloudInstance || ''}${
            configs.authConfig?.azureAuth?.tenantId || ''
        }`,
        redirectUri: configs.authConfig?.azureAuth?.redirectURL || '',
    },
    cache: {
        cacheLocation: 'sessionStorage',
        storeAuthStateInCookie: false,
    },
});

// - Email scope was added later so kuberpult can extract the email from requests (the author)
//   and send it along to the backend
const loginRequest = {
    scopes: ['email'],
};

export const AcquireToken: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const { instance, accounts } = useMsal();

    React.useEffect(() => {
        const request = {
            ...loginRequest,
            account: accounts[0],
        };
        instance
            .acquireTokenSilent(request)
            .then((response) => {
                AzureAuthSub.set({
                    authHeader: new BrowserHeaders({ Authorization: response.idToken }),
                    authReady: true,
                });
            })
            .catch(() => {
                instance.acquireTokenPopup(request).then((response) => {
                    AzureAuthSub.set({
                        authHeader: new BrowserHeaders({ Authorization: response.idToken }),
                        authReady: true,
                    });
                });
            });
    }, [instance, accounts]);

    return <>{children}</>;
};

export const AzureAutoSignIn = (): JSX.Element => {
    const isAuthenticated = useIsAuthenticated();
    const { instance } = useMsal();
    React.useEffect(() => {
        if (!isAuthenticated) {
            instance.loginRedirect(loginRequest);
        }
    }, [instance, isAuthenticated]);
    return <>Redirecting to login</>;
};

export const AzureAuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const { configs, configReady } = useFrontendConfig((c) => c);
    const msalInstance = React.useMemo(() => new PublicClientApplication(getMsalConfig(configs)), [configs]);
    if (!configReady) return null;

    const useAzureAuth = configs.authConfig?.azureAuth?.enabled;
    if (!useAzureAuth) {
        AzureAuthSub.set({ authReady: true });
        return <>{children}</>;
    }

    return (
        <MsalProvider instance={msalInstance}>
            <AuthenticatedTemplate>
                <AcquireToken>{children}</AcquireToken>
            </AuthenticatedTemplate>
            <UnauthenticatedTemplate>
                <AzureAutoSignIn />
            </UnauthenticatedTemplate>
        </MsalProvider>
    );
};
