import { act, render, screen } from '@testing-library/react';
import { AuthProvider } from './AuthContext';
import { ConfigsContext } from './index';
import { Crypto } from '@peculiar/webcrypto';

describe('AuthProvider', () => {
    const getNode = ({ enableAzureAuth }: { enableAzureAuth: boolean }) => {
        const configs = {
            authConfig: {
                azureAuth: {
                    enabled: enableAzureAuth,
                    clientId: 'db7fc493-a2fd-4f49-aff3-0aec08f03516',
                    cloudInstance: 'https://login.microsoftonline.com/',
                    redirectURL: 'http://localhost:3000',
                    tenantId: '3912f51d-cb71-4c9f-b0b9-f55c90238dd9',
                },
            },
        };
        const setConfigs = () => {};
        return (
            <ConfigsContext.Provider value={{ configs, setConfigs }}>
                <AuthProvider>
                    <>Content</>
                </AuthProvider>
            </ConfigsContext.Provider>
        );
    };
    const getWrapper = ({ enableAzureAuth = false }) => render(getNode({ enableAzureAuth }));

    beforeAll(() => {
        Object.defineProperty(window, 'crypto', {
            value: new Crypto(),
        });
    });

    describe.each([
        {
            name: 'Shows unauthed content when not logged in',
            enableAzureAuth: true,
            content: 'Redirecting to login',
        },
        {
            name: 'Shows normal content when not auth not enabled',
            enableAzureAuth: false,
            content: 'Content',
        },
    ])(`Checkout`, (testcase: any) => {
        it(`${testcase.name}`, async () => {
            getWrapper(testcase);
            await act(async () => await global.nextTick());
            expect(screen.getByText(testcase.content)).toBeInTheDocument();
        });
    });
});
