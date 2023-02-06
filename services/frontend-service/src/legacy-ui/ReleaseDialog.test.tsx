import React from 'react';
import { getByTestId, render } from '@testing-library/react';
import ReleaseDialog, { ArgoCdLink, getFullUrl, getUndeployedUpstream } from './ReleaseDialog';
import { ActionsCartContext } from './App';
import { Environment } from '../api/api';

describe('VersionDiff', () => {
    it.each([
        {
            availableVersions: [1],
            deployedVersion: 1,
            targetVersion: 1,
            expectedLabel: 'same version',
        },
        {
            availableVersions: [1, 2, 3, 4],
            deployedVersion: 4,
            targetVersion: 1,
            expectedLabel: 'currently deployed: 3 ahead',
        },
        {
            availableVersions: [1, 14, 38, 139],
            deployedVersion: 139,
            targetVersion: 1,
            expectedLabel: 'currently deployed: 3 ahead',
        },
        {
            availableVersions: [1, 14, 38, 139],
            deployedVersion: 1,
            targetVersion: 139,
            expectedLabel: 'currently deployed: 3 behind',
        },
    ])('renders the correct version diff', ({ availableVersions, deployedVersion, targetVersion, expectedLabel }) => {
        const overview = {
            environments: {
                development: {
                    name: 'development',
                    locks: {},
                    applications: {
                        demo: {
                            name: 'demo',
                            version: deployedVersion,
                            locks: {},
                            team: 'testing',
                            sourceRepoUrl: 'git.test/repo',
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            applications: {
                demo: {
                    name: 'demo',
                    team: 'testing',
                    sourceRepoUrl: 'git.test/repo',
                    releases: availableVersions.map((v) => ({
                        version: v,
                        sourceCommitId: '',
                        sourceAuthor: '',
                        prNumber: '123',
                        sourceMessage: '',
                        undeployVersion: false,
                    })),
                },
            },
        };
        const app = render(
            <ActionsCartContext.Provider value={{ actions: [], setActions: () => null }}>
                <ReleaseDialog overview={overview} applicationName="demo" version={targetVersion} sortOrder={{}} />
            </ActionsCartContext.Provider>
        );

        const diff = app.getByTestId('version-diff');
        expect(diff).toHaveAttribute('aria-label', expectedLabel);
    });
});

describe('UndeployedUpstream', () => {
    it.each([
        {
            environment: 'Aone',
            version: 21,
            expectedUpstream: '',
        },
        {
            environment: 'Atwo',
            version: 20,
            expectedUpstream: '',
        },
        {
            environment: 'Atwo',
            version: 21,
            expectedUpstream: 'Aone',
        },
        {
            environment: 'Athree',
            version: 20,
            expectedUpstream: 'Atwo',
        },
        {
            environment: 'Bone',
            version: 23,
            expectedUpstream: '',
        },
        {
            environment: 'Btwo',
            version: 23,
            expectedUpstream: 'Bone',
        },
    ])('Gives correct undeployed upstream', ({ environment, version, expectedUpstream }) => {
        function getEnvironment(upstream: string, version: number) {
            return {
                config: {
                    upstream: {
                        upstream: {
                            $case: 'environment' as const,
                            environment: upstream,
                        },
                    },
                },
                applications: {
                    appName: {
                        name: 'appName',
                        queuedVersion: 0,
                        locks: {},
                        undeployVersion: false,
                        version: version,
                    },
                },
                name: '',
                locks: {},
            };
        }
        const environments: { [key: string]: Environment } = {
            Aone: getEnvironment('', 20),
            Atwo: getEnvironment('Aone', 19),
            Athree: getEnvironment('Atwo', 19),
            Bone: getEnvironment('', 22),
            Btwo: getEnvironment('Bone', 22),
        };
        const undeployedUpstream = getUndeployedUpstream(environments, environment, 'appName', version);
        expect(undeployedUpstream).toBe(expectedUpstream);
    });
});

describe('QueueDiff', () => {
    it.each([
        {
            targetVersion: 1,
            queuedVersion: 0,
            expectedLabel: '',
        },
        {
            targetVersion: 1,
            queuedVersion: 2,
            expectedLabel: '+1',
        },
        {
            targetVersion: 2,
            queuedVersion: 1,
            expectedLabel: '-1',
        },
        {
            targetVersion: 1,
            queuedVersion: 4,
            expectedLabel: '+3',
        },
    ])('renders the correct queue diff', ({ queuedVersion, targetVersion, expectedLabel }) => {
        const availableVersions = [1, 2, 3, 4];
        const overview = {
            environments: {
                development: {
                    name: 'development',
                    locks: {},
                    applications: {
                        demo: {
                            name: 'demo',
                            queuedVersion: queuedVersion,
                            locks: {},
                            team: 'testing',
                            sourceRepoUrl: 'git.test/repo',
                            version: 1,
                            undeployVersion: false,
                        },
                    },
                },
            },
            applications: {
                demo: {
                    name: 'demo',
                    team: 'testing',
                    sourceRepoUrl: 'git.test/repo',
                    releases: availableVersions.map((v) => ({
                        version: v,
                        sourceCommitId: '',
                        sourceAuthor: '',
                        sourceMessage: '',
                        prNumber: '123',
                        undeployVersion: false,
                    })),
                },
            },
        };
        const app = render(
            <ActionsCartContext.Provider value={{ actions: [], setActions: () => null }}>
                <ReleaseDialog overview={overview} applicationName="demo" version={targetVersion} sortOrder={{}} />
            </ActionsCartContext.Provider>
        );

        const diff = app.getByTestId('queue-diff');
        expect(diff.textContent).toEqual(expectedLabel);
    });
});

describe('ReleaseDialog', () => {
    describe.each([
        {
            argoCD: undefined,
        },
        {
            argoCD: {
                syncWindows: [],
            },
        },
        {
            argoCD: {
                syncWindows: [{ kind: 'allow', schedule: '* * * * *', duration: '0s' }],
            },
        },
    ])('renders warnings', ({ argoCD }) => {
        it(`for ${argoCD?.syncWindows.length ?? 'undefined'} sync windows`, () => {
            const overview = {
                environments: {
                    development: {
                        name: 'development',
                        locks: {},
                        applications: {
                            demo: {
                                name: 'demo',
                                version: 1,
                                locks: {},
                                team: 'testing',
                                sourceRepoUrl: 'git.test/repo',
                                queuedVersion: 1,
                                undeployVersion: false,
                                argoCD,
                            },
                        },
                    },
                },
                applications: {
                    demo: {
                        name: 'demo',
                        team: 'testing',
                        sourceRepoUrl: 'git.test/repo',
                        releases: [
                            {
                                version: 1,
                                sourceCommitId: '',
                                sourceAuthor: '',
                                prNumber: '123',
                                sourceMessage: '',
                                undeployVersion: false,
                            },
                        ],
                    },
                },
            };

            render(
                <ActionsCartContext.Provider value={{ actions: [], setActions: () => null }}>
                    <ReleaseDialog overview={overview} applicationName="demo" version={1} sortOrder={{}} />
                </ActionsCartContext.Provider>
            );

            const syncWindowElements = document.querySelectorAll('.syncWindow');
            expect(syncWindowElements).toHaveLength(argoCD?.syncWindows.length ?? 0);
        });
    });
});

describe('Argocd Link', () => {
    interface dataT {
        name: string;
        baseUrl: string;
        applicationName: string;
        environmentName: string;
        expect: (container: HTMLElement, url?: string) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'renders the UndeployBtn component',
            baseUrl: '',
            applicationName: 'app1',
            environmentName: 'env1',
            expect: (container, url?: string) =>
                expect(container.querySelector('.MuiButtonBase-root')).not.toBeTruthy(),
        },
        {
            name: 'renders the ArgoCd component with baseUrl',
            baseUrl: 'http://my-awsome-site.xyz',
            applicationName: 'app1', //
            environmentName: 'env1',
            expect: (container, url?: string) =>
                expect(getByTestId(container, 'argocd-link')).toHaveAttribute('aria-label', url),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <ArgoCdLink {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { baseUrl: string; applicationName: string; environmentName: string }) =>
        render(getNode(overrides));

    describe.each(data)(`Argocd Link btn`, (testcase) => {
        it(testcase.name, () => {
            const { applicationName, baseUrl, environmentName } = testcase;
            // when
            const { container } = getWrapper({ baseUrl, applicationName, environmentName });
            // then
            const url = getFullUrl(baseUrl, environmentName, applicationName);
            testcase.expect(container, url);
        });
    });
});
