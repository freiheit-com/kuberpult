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
import { act, getByText, render, screen, waitFor } from '@testing-library/react';
import { AcquireToken, AzureAuthProvider, AzureAuthSub, AzureAutoSignIn, Utf8ToBase64 } from './AzureAuthProvider';
import { Crypto } from '@peculiar/webcrypto';
import { PublicClientApplication, IPublicClientApplication, Configuration, AccountInfo } from '@azure/msal-browser';
import { AuthenticatedTemplate, MsalProvider, UnauthenticatedTemplate } from '@azure/msal-react';
import { AuthenticationResult } from '@azure/msal-common';
import { UpdateFrontendConfig } from './store';
import { BrowserHeaders } from 'browser-headers';

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
        username: 'mail@example.com',
        name: 'test person',
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
        it('Get id token and store it in authHeader with acquireTokenSilent method', async () => {
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
            await waitFor(() =>
                expect(AzureAuthSub.get().authHeader.get('author-email')).toContain(Utf8ToBase64('mail@example.com'))
            );
            await waitFor(() =>
                expect(AzureAuthSub.get().authHeader.get('author-name')).toContain(Utf8ToBase64('test person'))
            );
        });

        it('Get id token and store it in authHeader with acquireTokenPopup method', async () => {
            // given
            jest.spyOn(pca, 'getAllAccounts').mockImplementation(() => [testAccount]);
            const acquireTokenSilentSpy = jest
                .spyOn(pca, 'acquireTokenSilent')
                .mockImplementation((r) => Promise.reject(new Error('promise failed testing')));
            const acquireTokenPopup = jest
                .spyOn(pca, 'acquireTokenPopup')
                .mockImplementation((r) => Promise.resolve(makeAuthenticationResult({ idToken: 'unique-token-2' })));

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
            await waitFor(async () => expect(acquireTokenPopup).toHaveBeenCalledTimes(1));
            await waitFor(() => expect(AzureAuthSub.get().authHeader.get('authorization')).toContain('unique-token-2'));
            await waitFor(() =>
                expect(AzureAuthSub.get().authHeader.get('author-email')).toContain(Utf8ToBase64('mail@example.com'))
            );
            await waitFor(() =>
                expect(AzureAuthSub.get().authHeader.get('author-name')).toContain(Utf8ToBase64('test person'))
            );
        });
        it('Get id token both method failed', async () => {
            // given
            AzureAuthSub.set({
                authHeader: new BrowserHeaders({}),
                authReady: false,
            });
            // eslint-disable-next-line no-console
            console.error = jest.fn();

            jest.spyOn(pca, 'getAllAccounts').mockImplementation(() => [testAccount]);
            const acquireTokenSilentSpy = jest
                .spyOn(pca, 'acquireTokenSilent')
                .mockImplementation((r) => Promise.reject(new Error('promise failed testing 1')));
            const acquireTokenPopup = jest
                .spyOn(pca, 'acquireTokenPopup')
                .mockImplementation((r) => Promise.reject(new Error('promise failed testing 2')));

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
            await waitFor(async () => expect(acquireTokenSilentSpy).toHaveBeenCalledTimes(1));
            await waitFor(async () => expect(acquireTokenPopup).toHaveBeenCalledTimes(1));
            // eslint-disable-next-line no-console
            expect(console.error).toHaveBeenCalledWith(
                'acquireTokenSilent failed: ',
                new Error('promise failed testing 1')
            );
            // eslint-disable-next-line no-console
            expect(console.error).toHaveBeenCalledWith(
                'acquireTokenPopup failed: ',
                new Error('promise failed testing 2')
            );
            expect(screen.queryByText('Token Acquired')).not.toBeInTheDocument();
            expect(screen.queryByText('loading...')).toBeInTheDocument();
        });
    });

    describe('Azure Not Enabled Test', () => {
        it('Provider can display content when azure auth is off', () => {
            // given
            UpdateFrontendConfig.set({
                configs: {
                    manifestRepoUrl: 'myrepo',
                    sourceRepoUrl: 'mysource',
                    branch: 'main',
                    kuberpultVersion: '1.2.3',
                    authConfig: {
                        azureAuth: {
                            enabled: false,
                            clientId: 'none',
                            tenantId: 'no-tenant',
                            cloudInstance: 'myinstance',
                            redirectUrl: 'example.com',
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
    describe('Spinner renders', () => {
        it('Spinner is rendered when the config is not loaded', () => {
            // given
            UpdateFrontendConfig.set({
                configReady: false,
            });

            // when
            const { container } = render(
                <AzureAuthProvider>
                    <div className={'welcome-kuberpult-test'}>Welcome to kuberpult</div>
                </AzureAuthProvider>
            );

            // then
            expect(container.getElementsByClassName('spinner')).toHaveLength(1);
        });
    });
});
