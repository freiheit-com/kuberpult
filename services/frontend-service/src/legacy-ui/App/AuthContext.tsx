import * as React from 'react';
import { ConfigsContext } from './index';
import { Configuration, PublicClientApplication } from '@azure/msal-browser';
import { MsalProvider, AuthenticatedTemplate, useIsAuthenticated, UnauthenticatedTemplate } from '@azure/msal-react';

type AuthContextType = {};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

const getMsalConfig = (configs: any): Configuration => ({
    auth: {
        clientId: configs?.authConfig?.azureAuth?.clientId,
        authority: `https://login.microsoftonline.com/${configs?.authConfig?.azureAuth?.tenantId}`,
        // TODO: change to live url
        redirectUri: 'http://localhost:3000/account',
    },
    cache: {
        cacheLocation: 'sessionStorage',
        storeAuthStateInCookie: false,
    },
});

// const getLoginRequest = (configs: any) => ({
// scopes: ['User.Read'],
// });

export function AuthProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const { configs } = React.useContext(ConfigsContext);
    const isAuthenticated = useIsAuthenticated();
    const useAzureAuth = configs?.authConfig?.azureAuth?.enabled || false;
    const msalConfig = React.useMemo(() => getMsalConfig(configs), [configs]);
    // const loginRequest = React.useMemo(() => getLoginRequest(configs), [configs]);
    const msalInstance = new PublicClientApplication(msalConfig);
    if (useAzureAuth && !isAuthenticated) {
        // msalInstance.loginRedirect(loginRequest).catch(() => {
        // console.error(e);
        // });
        // eslint-disable-next-line no-console
        console.log(useAzureAuth, isAuthenticated, 'not authed');
    } else {
        // eslint-disable-next-line no-console
        console.log(useAzureAuth, isAuthenticated, 'authed');
    }

    return (
        <AuthContext.Provider value={{}}>
            {useAzureAuth ? (
                <MsalProvider instance={msalInstance}>
                    <AuthenticatedTemplate>{children}</AuthenticatedTemplate>
                    <UnauthenticatedTemplate></UnauthenticatedTemplate>
                </MsalProvider>
            ) : (
                children
            )}
        </AuthContext.Provider>
    );
}
