import * as React from 'react';
import { ConfigsContext } from './index';

type AuthContextType = {};

const AuthContext = React.createContext<AuthContextType>({} as AuthContextType);

export function AuthProvider({ children }: { children: React.ReactNode }): JSX.Element {
    const { configs } = React.useContext(ConfigsContext);
    const useAuth = configs?.authConfig?.azureAuth?.enabled || false;
    const [msalConfig, setMsalConfig] = React.useState({});
    const [loginRequest, setLoginRequest] = React.useState({});

    if (useAuth) {
        setMsalConfig({
            auth: {
                clientId: configs?.authConfig?.azureAuth?.clientId,
                authority: `https://login.microsoftonline.com/${configs?.authConfig?.azureAuth?.tenantId}`, // This is a URL (e.g. https://login.microsoftonline.com/{your tenant ID})
                redirectUri: 'http://localhost:3000/account',
            },
            cache: {
                cacheLocation: 'sessionStorage',
                storeAuthStateInCookie: false,
            },
        });
        setLoginRequest({
            scopes: ['User.Read'],
        });
    }

    // eslint-disable-next-line no-console
    console.log(msalConfig, loginRequest);
    // eslint-disable-next-line no-console
    console.log(useAuth);
    return <AuthContext.Provider value={{}}>{useAuth ? <>use auth instead </> : children}</AuthContext.Provider>;
}
