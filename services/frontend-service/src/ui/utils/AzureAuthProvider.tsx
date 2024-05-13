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
import * as React from 'react';
import { BrowserHeaders } from 'browser-headers';
import { Configuration, PublicClientApplication } from '@azure/msal-browser';
import {
    AuthenticatedTemplate,
    MsalProvider,
    UnauthenticatedTemplate,
    useIsAuthenticated,
    useMsal,
} from '@azure/msal-react';
import { GetFrontendConfigResponse } from '../../api/api';
import { createStore } from 'react-use-sub';
import { grpc } from '@improbable-eng/grpc-web';
import { useFrontendConfig } from './store';
import { AuthenticationResult } from '@azure/msal-common';
import { Spinner } from '../components/Spinner/Spinner';

export type AuthHeader = grpc.Metadata & {
    Authorization?: String;
};
type AzureAuthSubType = {
    authHeader: AuthHeader;
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
        redirectUri: configs.authConfig?.azureAuth?.redirectUrl || '',
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

// exported just for testing
export const Utf8ToBase64 = (str: string): string => window.btoa(unescape(encodeURIComponent(str)));

export const AcquireToken: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const { instance, accounts } = useMsal();

    const { authReady } = useAzureAuthSub((auth) => auth);

    React.useEffect(() => {
        const request = {
            ...loginRequest,
            account: accounts[0],
        };
        const email: string = Utf8ToBase64(accounts[0]?.username || ''); // yes, the email is in the "username" field
        const username: string = Utf8ToBase64(accounts[0]?.name || '');
        instance
            .acquireTokenSilent(request)
            .then((response: AuthenticationResult) => {
                AzureAuthSub.set({
                    authHeader: new BrowserHeaders({
                        Authorization: response.idToken,
                        'author-email': email, // use same key here as in server.go function getRequestAuthorFromAzure: r.Header.Get("email")
                        'author-name': username, // use same key here too
                    }),
                    authReady: true,
                });
            })
            .catch((silentError) => {
                instance
                    .acquireTokenPopup(request)
                    .then((response: AuthenticationResult) => {
                        AzureAuthSub.set({
                            authHeader: new BrowserHeaders({
                                Authorization: response.idToken,
                                'author-email': email, // use same key here as in server.go function getRequestAuthorFromAzure: r.Header.Get("email")
                                'author-name': username, // use same key here too
                            }),
                            authReady: true,
                        });
                    })
                    .catch((error) => {
                        // eslint-disable-next-line no-console
                        console.error('acquireTokenSilent failed: ', silentError);
                        // eslint-disable-next-line no-console
                        console.error('acquireTokenPopup failed: ', error);
                    });
            });
    }, [instance, accounts]);

    if (!authReady) {
        return <div>loading...</div>;
    }
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
    if (!configReady) {
        return <Spinner message={'Loading Configuration'} />;
    }

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
