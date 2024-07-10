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
import { jwtDecode } from 'jwt-decode';
//import Cookies from 'js-cookie';
import { TopAppBar } from '../components/TopAppBar/TopAppBar';
import { Button } from '../components/button';
import React from 'react';

export const LoginPage: React.FC = () => {
    // Define the navigation function using useCallback
    const handleRedirect = React.useCallback(() => {
        // Hard reload to invalidate cache and make the Dex call work.
        window.location.reload();
        window.location.href = '/login';
    }, []);

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content">
                <div className="login-page">
                    <h1 className="login-name">{'Login Into Dex'}</h1>
                    <div className="space_apart_row">
                        <Button
                            label={'Login'}
                            className="login_button"
                            onClick={handleRedirect}
                            highlightEffect={false}
                        />
                    </div>
                </div>
            </main>
        </div>
    );
};

// Validates the token expiring date
export function isTokenValid(): boolean {
    //const token = Cookies.get('kuberpult.oauth');
    const cookieValue = document.cookie
        .split('; ')
        .find((row) => row.startsWith('kuberpult.oauth='))
        ?.split('=')[1];
    if (!cookieValue) return false;

    const decodedToken = jwtDecode(cookieValue);
    if (!decodedToken) return false;

    const currentTime = Date.now() / 1000;
    if (!decodedToken.exp) return false;
    if (decodedToken.exp < currentTime) {
        return false;
    }
    return true;
}
