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

export const [useFrontendConfig, UpdateFrontendConfig] = createStore({
    configs: {} as GetFrontendConfigResponse,
    configReady: false,
});

type AzureAuthSubType = {
    authHeader: grpc.Metadata & {
        Authorization?: String;
    };
    authReady: boolean;
};
export const [useAzureAuthSub, AzureAuthSub] = createStore({
    authHeader: new BrowserHeaders({}),
    authReady: false,
} as AzureAuthSubType);

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

// - User.Read scope is required for the authentication. (checking that this user has access or not)
//   if user has access to our application the Token can be acquired.
//   More info: https://learn.microsoft.com/en-us/azure/active-directory/develop/scenario-spa-acquire-token?tabs=react
// - Email scope was added later so kuberpult can extract the email from requests (the author)
//   and send it along to the backend
const loginRequest = {
    scopes: ['User.Read', 'email'],
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

export const AzureAutoSignIn = () => {
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
