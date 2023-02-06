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
import { grpc } from '@improbable-eng/grpc-web';
import { BrowserHeaders } from 'browser-headers';
import { ConfigsContext } from './index';
import { Configuration, PublicClientApplication } from '@azure/msal-browser';
import {
    MsalProvider,
    AuthenticatedTemplate,
    useIsAuthenticated,
    UnauthenticatedTemplate,
    useMsal,
} from '@azure/msal-react';
import refreshStore from './RefreshStore';

type AuthContextType = {
    useAzureAuth: boolean;
    useAuth: boolean;
    msalConfig: Configuration;
};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

const getMsalConfig = (configs: any): Configuration => ({
    auth: {
        clientId: configs?.authConfig?.azureAuth?.clientId || '',
        authority: `${configs?.authConfig?.azureAuth?.cloudInstance || ''}${
            configs?.authConfig?.azureAuth?.tenantId || ''
        }`,
        redirectUri: configs?.authConfig?.azureAuth?.redirectURL || '',
    },
    cache: {
        cacheLocation: 'sessionStorage',
        storeAuthStateInCookie: false,
    },
});

const getLoginRequest = () => ({
    scopes: ['User.Read', 'email'],
});

export type AuthHeaderType = grpc.Metadata & {
    Authorization?: String;
};

type AuthTokenContextType = {
    token: String;
    authHeader: AuthHeaderType;
};

export const AuthTokenContext = React.createContext<AuthTokenContextType>({} as AuthTokenContextType);

function AzureAuthTokenProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const loginRequest = React.useMemo(() => getLoginRequest(), []);
    const [token, setToken] = React.useState('');
    const [authHeader, setAuthHeader] = React.useState(new BrowserHeaders({}));
    const { instance, accounts } = useMsal();

    React.useEffect(() => {
        const request = {
            ...loginRequest,
            account: accounts[0],
        };
        instance
            .acquireTokenSilent(request)
            .then((response) => {
                setToken(response.idToken);
                setAuthHeader(new BrowserHeaders({ Authorization: response.idToken }));
                refreshStore.setRefresh(true);
            })
            .catch(() => {
                instance.acquireTokenPopup(request).then((response) => {
                    setToken(response.idToken);
                    setAuthHeader(new BrowserHeaders({ Authorization: response.idToken }));
                    refreshStore.setRefresh(true);
                });
            });
    }, [instance, accounts, loginRequest]);
    return <AuthTokenContext.Provider value={{ token, authHeader }}>{children}</AuthTokenContext.Provider>;
}

function MsalProviderWrapper({ children }: { children: React.ReactNode }): JSX.Element {
    const { msalConfig } = React.useContext(AuthContext);
    const msalInstance = React.useMemo(() => new PublicClientApplication(msalConfig), [msalConfig]);
    return <MsalProvider instance={msalInstance}>{children}</MsalProvider>;
}

export const AzureAutoSignIn = () => {
    const isAuthenticated = useIsAuthenticated();
    const loginRequest = React.useMemo(() => getLoginRequest(), []);
    const { instance } = useMsal();
    React.useEffect(() => {
        if (!isAuthenticated) {
            instance.loginRedirect(loginRequest);
        }
    }, [instance, isAuthenticated, loginRequest]);
    return <>Redirecting to login</>;
};

export function AuthProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const { configs } = React.useContext(ConfigsContext);
    const useAzureAuth = React.useMemo(() => configs?.authConfig?.azureAuth?.enabled || false, [configs]);
    const useAuth = React.useMemo(() => useAzureAuth, [useAzureAuth]);
    const msalConfig = React.useMemo(() => getMsalConfig(configs), [configs]);

    return (
        <>
            {!!configs && Object.keys(configs).length > 0 && (
                <AuthContext.Provider value={{ useAzureAuth, useAuth, msalConfig }}>
                    {useAzureAuth ? (
                        <MsalProviderWrapper>
                            <AuthenticatedTemplate>
                                <AzureAuthTokenProvider>{children}</AzureAuthTokenProvider>
                            </AuthenticatedTemplate>
                            <UnauthenticatedTemplate>
                                <AzureAutoSignIn />
                            </UnauthenticatedTemplate>
                        </MsalProviderWrapper>
                    ) : (
                        <AuthTokenContext.Provider value={{ token: '', authHeader: new BrowserHeaders({}) }}>
                            {children}
                        </AuthTokenContext.Provider>
                    )}
                </AuthContext.Provider>
            )}
        </>
    );
}
