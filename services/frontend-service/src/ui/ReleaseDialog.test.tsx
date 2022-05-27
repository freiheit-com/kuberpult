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
import ReleaseDialog from './ReleaseDialog';
import { ActionsCartContext } from './App';

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
                        },
                    },
                },
            },
            applications: {
                demo: {
                    name: 'demo',
                    releases: availableVersions.map((v) => ({
                        version: v,
                    })),
                },
            },
        };
        const app = render(
            <ActionsCartContext.Provider value={{ actions: [] }}>
                <ReleaseDialog overview={overview} applicationName="demo" version={targetVersion} />
            </ActionsCartContext.Provider>
        );

        const diff = app.getByTestId('version-diff');
        expect(diff).toHaveAttribute('aria-label', expectedLabel);
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
                    })),
                },
            },
        };
        const app = render(
            <ActionsCartContext.Provider value={{ actions: [] }}>
                <ReleaseDialog overview={overview} applicationName="demo" version={targetVersion} />
            </ActionsCartContext.Provider>
        );

        const diff = app.getByTestId('queue-diff');
        expect(diff.textContent).toEqual(expectedLabel);
    });
});
