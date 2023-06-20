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
import { act, getByText, render, screen, waitFor } from '@testing-library/react';
import {
    AcquireToken,
    AzureAuthProvider,
    AzureAuthSub,
    AzureAutoSignIn,
    UpdateFrontendConfig,
} from './AzureAuthProvider';
import { Crypto } from '@peculiar/webcrypto';
import { PublicClientApplication, IPublicClientApplication, Configuration, AccountInfo } from '@azure/msal-browser';
import { AuthenticatedTemplate, MsalProvider, UnauthenticatedTemplate } from '@azure/msal-react';
import { AuthenticationResult } from '@azure/msal-common';

const makeAuthenticationResult = (partial: Partial<AuthenticationResult>): AuthenticationResult => ({
    authority: 'authority',
    uniqueId: 'unqueId',
    tenantId: 'tenantId',
    scopes: [],
    account: null,
    idToken: 'idToken',
    idTokenClaims: {},
    accessToken: 'accessToken',
    fromCache: false,
    expiresOn: null,
    tokenType: 'tokenType',
    correlationId: 'correlationId',
    ...partial,
});

describe('AuthProvider', () => {
    let pca: IPublicClientApplication;
    const clientId = 'db7fc493-a2fd-4f49-aff3-0aec08f03516';
    const tenantId = '3912f51d-cb71-4c9f-b0b9-f55c90238dd9';
    const testAccount: AccountInfo = {
        homeAccountId: '',
        localAccountId: '',
        environment: '',
        tenantId,
        username: '',
    };
    const msalConfig: Configuration = {
        auth: {
            clientId,
        },
    };

    beforeAll(() => {
        Object.defineProperty(window, 'crypto', {
            value: new Crypto(),
        });
    });

    beforeEach(() => {
        pca = new PublicClientApplication(msalConfig);
        global.document.cookie = '';
    });

    afterEach(() => {
        // cleanup on exiting
        jest.clearAllMocks();
    });

    describe('Authenticated', () => {
        it('Show Authenticated template when logged in', async () => {
            const handleRedirectSpy = jest.spyOn(pca, 'handleRedirectPromise');
            const getAllAccountsSpy = jest.spyOn(pca, 'getAllAccounts');
            getAllAccountsSpy.mockImplementation(() => [testAccount]);
            render(
                <MsalProvider instance={pca}>
                    <AuthenticatedTemplate>Authenticated</AuthenticatedTemplate>
                </MsalProvider>
            );
            await act(async (): Promise<void> => await global.nextTick());
            await waitFor(() => expect(handleRedirectSpy).toHaveBeenCalledTimes(1));
            expect(screen.getByText('Authenticated')).toBeInTheDocument();
        });
    });

    describe('AzureAutoSignIn', () => {
        it('Redirect when not logged in', async () => {
            const handleRedirectSpy = jest.spyOn(pca, 'handleRedirectPromise');
            const getAllAccountsSpy = jest.spyOn(pca, 'getAllAccounts');
            const redirectSpy = jest.spyOn(pca, 'loginRedirect');
            getAllAccountsSpy.mockImplementation(() => []);
            render(
                <MsalProvider instance={pca}>
                    <UnauthenticatedTemplate>
                        <AzureAutoSignIn />
                    </UnauthenticatedTemplate>
                </MsalProvider>
            );
            await act(async () => await global.nextTick());
            await waitFor(() => expect(redirectSpy).toHaveBeenCalledTimes(1));
            await waitFor(() => expect(handleRedirectSpy).toHaveBeenCalledTimes(1));
        });
    });

    describe('AcquireToken', () => {
        it('Get id token and store it in authHeader', async () => {
            // given
            jest.spyOn(pca, 'getAllAccounts').mockImplementation(() => [testAccount]);
            const acquireTokenSilentSpy = jest
                .spyOn(pca, 'acquireTokenSilent')
                .mockImplementation((r) => Promise.resolve(makeAuthenticationResult({ idToken: 'unique-token' })));

            // when
            render(
                <MsalProvider instance={pca}>
                    <AuthenticatedTemplate>
                        <AcquireToken>
                            <span>Token Acquired</span>
                        </AcquireToken>
                    </AuthenticatedTemplate>
                </MsalProvider>
            );
            await act(async () => await global.nextTick());

            // then
            expect(screen.queryByText('Token Acquired')).toBeInTheDocument();
            await waitFor(async () => expect(acquireTokenSilentSpy).toHaveBeenCalledTimes(1));
            await waitFor(() => expect(AzureAuthSub.get().authHeader.get('authorization')).toContain('unique-token'));
        });
    });

    describe('Azure Not Enabled Test', () => {
        it('Provider can display content when azure auth is off', () => {
            // given
            UpdateFrontendConfig.set({
                configs: {
                    sourceRepoUrl: 'myrepo',
                    kuberpultVersion: '1.2.3',
                    authConfig: {
                        azureAuth: {
                            enabled: false,
                            clientId: 'none',
                            tenantId: 'no-tenant',
                            cloudInstance: 'myinstance',
                            redirectURL: 'example.com',
                        },
                    },
                },
                configReady: true,
            });

            // when
            const { container } = render(
                <AzureAuthProvider>
                    <div className={'welcome-kuberpult-test'}>Welcome to kuberpult</div>
                </AzureAuthProvider>
            );

            // then
            expect(getByText(container, /Welcome to kuberpult/i)).toBeInTheDocument();
        });
    });
});
