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
import { render } from '@testing-library/react';
import { UpdateOverview, updateManifestInfo, ManifestResponse, ManifestRequestState } from '../../utils/store';
import { MemoryRouter } from 'react-router-dom';

import { fakeLoadEverything, enableDexAuth } from '../../../setupTests';
import { ManifestsPage } from './ManifestsPage';
import { Manifest, Priority } from '../../../api/api';

const targetApp = 'appName';

const targetReleaseVersion = '1';
const targetReleaseRevision = '0';
jest.mock('react-router-dom', () => ({
    ...jest.requireActual('react-router-dom'),
    useSearchParams: () => [
        new URLSearchParams({ app: targetApp, release: targetReleaseVersion, revision: targetReleaseRevision }),
    ],
}));
describe('Manifests Page', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <ManifestsPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());
    it('Renders full app', () => {
        fakeLoadEverything(true);
        updateManifestInfo.set({
            response: undefined,
            manifestInfoReady: ManifestRequestState.NOTFOUND,
        });
        const { container } = getWrapper();
        expect(container.getElementsByClassName('manifests-page')[0]).toBeInTheDocument();
    });
    it('Renders login page if Dex enabled', () => {
        fakeLoadEverything(true);
        enableDexAuth(false);
        const { container } = getWrapper();
        expect(container.getElementsByClassName('environment_name')[0]).toHaveTextContent('Log in to Dex');
    });
    it('Renders page page if Dex enabled and valid token', () => {
        fakeLoadEverything(true);
        enableDexAuth(true);
        updateManifestInfo.set({
            response: undefined,
            manifestInfoReady: ManifestRequestState.NOTFOUND,
        });

        const { container } = getWrapper();
        expect(container.getElementsByClassName('manifests-page')[0]).toBeInTheDocument();
    });
    it('Renders spinner', () => {
        // given
        UpdateOverview.set({
            loaded: false,
        });
        // when
        const { container } = getWrapper();
        // then
        expect(container.getElementsByClassName('spinner')).toHaveLength(1);
    });
});

describe('Test Manifests', () => {
    const getNode = (): JSX.Element | any => (
        <MemoryRouter>
            <ManifestsPage />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());
    const envName1 = 'env-1';
    const envName2 = 'env-2';
    interface dataEnvT {
        name: string;
        app: string;
        releaseVersion: string;
        response: ManifestResponse;
        expectedMessage: string;
    }

    const errorTestData: dataEnvT[] = [
        {
            name: 'no manifests found',
            app: targetApp,
            releaseVersion: targetReleaseVersion,
            response: {
                response: undefined,
                manifestInfoReady: ManifestRequestState.NOTFOUND,
            },
            expectedMessage: 'Kuberpult could not find the manifests for release 1 of appName.',
        },
        {
            name: 'error fetching manifests',
            app: targetApp,
            releaseVersion: targetReleaseVersion,
            response: {
                response: undefined,
                manifestInfoReady: ManifestRequestState.ERROR,
            },
            expectedMessage: 'Something went wrong fetching data from Kuberpult.',
        },
    ];

    describe.each(errorTestData)(`Test Manifests errors`, (testcase) => {
        it(testcase.name, () => {
            // given
            fakeLoadEverything(true);

            updateManifestInfo.set(testcase.response);

            const { container } = getWrapper();
            expect(container.getElementsByClassName('manifests-page')[0]).toHaveTextContent(testcase.expectedMessage);
        });
    });

    const testData: dataEnvT[] = [
        {
            name: 'one manifest for one environment',
            app: targetApp,
            releaseVersion: targetReleaseVersion,
            response: {
                response: {
                    release: {
                        version: parseInt(targetReleaseVersion),
                        sourceCommitId: 'something',
                        sourceAuthor: 'some author',
                        sourceMessage: 'some message',
                        undeployVersion: false,
                        prNumber: '',
                        displayVersion: '',
                        isMinor: false,
                        isPrepublish: false,
                        environments: [],
                        ciLink: '',
                        revision: 0,
                    },
                    manifests: {
                        'env-1': {
                            content: 'envName1 content',
                            environment: envName1,
                        },
                    },
                },
                manifestInfoReady: ManifestRequestState.READY,
            },
            expectedMessage: '',
        },
        {
            name: 'mulitple manifests ',
            app: targetApp,
            releaseVersion: targetReleaseVersion,
            response: {
                response: {
                    release: {
                        version: parseInt(targetReleaseVersion),
                        sourceCommitId: 'something',
                        sourceAuthor: 'some author',
                        sourceMessage: 'some message',
                        undeployVersion: false,
                        prNumber: '',
                        displayVersion: '',
                        isMinor: false,
                        isPrepublish: false,
                        environments: [],
                        ciLink: '',
                        revision: 0,
                    },
                    manifests: {
                        'env-1': {
                            content: 'envName1 content',
                            environment: envName1,
                        },
                        'env-2': {
                            content: 'envName2 content',
                            environment: envName1,
                        },
                    },
                },
                manifestInfoReady: ManifestRequestState.READY,
            },
            expectedMessage: '',
        },
    ];

    describe.each(testData)(`Test Manifests`, (testcase) => {
        it(testcase.name, () => {
            // given
            fakeLoadEverything(true);
            UpdateOverview.set({
                lightweightApps: [], //does not matter
                environmentGroups: [
                    {
                        environments: [
                            {
                                name: envName1,
                                priority: Priority.UNRECOGNIZED,
                                distanceToUpstream: 0,
                            },
                            {
                                name: envName2,
                                priority: Priority.UNRECOGNIZED,
                                distanceToUpstream: 0,
                            },
                        ],
                        environmentGroupName: 'dontcare',
                        distanceToUpstream: 0,
                        priority: Priority.UNRECOGNIZED,
                    },
                ],
            });
            updateManifestInfo.set(testcase.response);

            const { container } = getWrapper();
            expect(container.getElementsByClassName('manifests-page')[0]).toHaveTextContent(
                'Manifests for release ' + targetReleaseVersion + " of '" + targetApp + "'"
            );
            if (testcase.response.response) {
                const allManifests = new Map<string, Manifest>(Object.entries(testcase.response.response.manifests));
                allManifests.forEach((expectedManifest, currentManifestEnv) => {
                    expect(document.getElementById('manifest-' + currentManifestEnv)).toHaveTextContent(
                        expectedManifest.content
                    );
                });
            }
        });
    });
});
