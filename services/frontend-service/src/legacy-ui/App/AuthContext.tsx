/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import * as React from 'react';
import { ConfigsContext } from './index';
import { Configuration, PublicClientApplication } from '@azure/msal-browser';
import {
    MsalProvider,
    AuthenticatedTemplate,
    useIsAuthenticated,
    UnauthenticatedTemplate,
    useMsal,
} from '@azure/msal-react';

type AuthContextType = {
    useAzureAuth: boolean;
};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

const getMsalConfig = (configs: any): Configuration => ({
    auth: {
        clientId: configs?.authConfig?.azureAuth?.clientId,
        authority: `${configs?.authConfig?.azureAuth?.cloudInstance}${configs?.authConfig?.azureAuth?.tenantId}`,
        redirectUri: configs?.authConfig?.azureAuth?.redirectURL,
    },
    cache: {
        cacheLocation: 'sessionStorage',
        storeAuthStateInCookie: false,
    },
});

const getLoginRequest = () => ({
    scopes: ['User.Read'],
});

type AuthTokenContextType = {
    token: String;
};

export const AuthTokenContext = React.createContext<AuthTokenContextType>({} as AuthTokenContextType);

function AuthTokenProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const { useAzureAuth } = React.useContext(AuthContext);
    const { instance, accounts } = useMsal();
    const loginRequest = React.useMemo(() => getLoginRequest(), []);
    const [token, setToken] = React.useState('');
    if (useAzureAuth) {
        const request = {
            ...loginRequest,
            account: accounts[0],
        };
        instance
            .acquireTokenSilent(request)
            .then((response) => {
                setToken(response.accessToken);
            })
            .catch(() => {
                instance.acquireTokenPopup(request).then((response) => {
                    setToken(response.accessToken);
                });
            });
    }
    return <AuthTokenContext.Provider value={{ token }}>{children}</AuthTokenContext.Provider>;
}

const AzureAutoSignIn = () => {
    const isAuthenticated = useIsAuthenticated();
    const loginRequest = React.useMemo(() => getLoginRequest(), []);
    const { instance } = useMsal();
    if (!isAuthenticated) {
        instance.loginRedirect(loginRequest);
    }
    return <></>;
};

export function AuthProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const { configs } = React.useContext(ConfigsContext);
    const useAzureAuth = configs?.authConfig?.azureAuth?.enabled || false;
    const msalConfig = React.useMemo(() => getMsalConfig(configs), [configs]);
    const msalInstance = new PublicClientApplication(msalConfig);

    return (
        <>
            {!!configs && Object.keys(configs).length > 0 && (
                <AuthContext.Provider value={{ useAzureAuth }}>
                    {useAzureAuth ? (
                        <MsalProvider instance={msalInstance}>
                            <AuthenticatedTemplate>
                                <AuthTokenProvider>{children}</AuthTokenProvider>
                            </AuthenticatedTemplate>
                            <UnauthenticatedTemplate>
                                <AzureAutoSignIn />
                            </UnauthenticatedTemplate>
                        </MsalProvider>
                    ) : (
                        <AuthTokenProvider>{children}</AuthTokenProvider>
                    )}
                </AuthContext.Provider>
            )}
        </>
    );
}
