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
import { TopAppBar } from '../components/TopAppBar/TopAppBar';
import { Button } from '../components/button';
import React from 'react';

export const LoginPage: React.FC = () => {
    // Redirect the user to the Dex Login
    const handleRedirect = React.useCallback(() => {
        // A random value is added to avoid browser cashing when redirecting to DEX. 
        const id = window.crypto.randomUUID()
        window.location.href = `/login?random_value=${id}`;
    }, []);

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <main className="main-content">
                <h1 className="environment_name">{'Log in to Dex'}</h1>
                <h3 className="page_description">
                    {'You are currently not logged in. Please log in to continue.'}
                </h3>
                <div className="space_apart_row">
                    <Button
                        label={'Login'}
                        className={'button-main env-card-deploy-btn mdc-button--unelevated'}
                        onClick={handleRedirect}
                        highlightEffect={false}
                    />
                </div>
            </main>
        </div>
    );
};

// Validates the token expiring date
export function isTokenValid(): boolean {
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
