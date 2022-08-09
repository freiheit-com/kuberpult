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

type AuthContextType = {};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

const getMsalConfig = (configs: any): Configuration => ({
    auth: {
        clientId: configs?.authConfig?.azureAuth?.clientId,
        authority: `https://login.microsoftonline.com/${configs?.authConfig?.azureAuth?.tenantId}`,
        // TODO: change to live url
        redirectUri: 'http://localhost:3000',
    },
    cache: {
        cacheLocation: 'sessionStorage',
        storeAuthStateInCookie: false,
    },
});

const getLoginRequest = () => ({
    scopes: ['User.Read'],
});

const AutoSignIn = (props: {}) => {
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
        <AuthContext.Provider value={{}}>
            {useAzureAuth ? (
                <MsalProvider instance={msalInstance}>
                    <AuthenticatedTemplate>{children}</AuthenticatedTemplate>
                    <UnauthenticatedTemplate>
                        <AutoSignIn />
                    </UnauthenticatedTemplate>
                </MsalProvider>
            ) : (
                children
            )}
        </AuthContext.Provider>
    );
}
