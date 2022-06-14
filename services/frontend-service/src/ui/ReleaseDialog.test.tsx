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
import React from 'react';
import { render } from '@testing-library/react';
import ReleaseDialog, { getUndeployedUpstream } from './ReleaseDialog';
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
                            queuedVersion: 0,
                            undeployVersion: false,
                        },
                    },
                },
            },
            applications: {
                demo: {
                    name: 'demo',
                    releases: availableVersions.map((v) => ({
                        version: v,
                        sourceCommitId: '',
                        sourceAuthor: '',
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
                            version: 1,
                            undeployVersion: false,
                        },
                    },
                },
            },
            applications: {
                demo: {
                    name: 'demo',
                    releases: availableVersions.map((v) => ({
                        version: v,
                        sourceCommitId: '',
                        sourceAuthor: '',
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
                        releases: [
                            {
                                version: 1,
                                sourceCommitId: '',
                                sourceAuthor: '',
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
