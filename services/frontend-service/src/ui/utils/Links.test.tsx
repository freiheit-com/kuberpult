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
import { render } from '@testing-library/react';
import React from 'react';
import {
    ArgoAppEnvLink,
    ArgoAppLink,
    ArgoTeamLink,
    DisplayManifestLink,
    DisplaySourceLink,
    DisplayCommitHistoryLink,
    KuberpultGitHubLink,
} from './Links';
import { GetFrontendConfigResponse_ArgoCD } from '../../api/api';
import { UpdateFrontendConfig } from './store';
import { elementQuerySelectorSafe } from '../../setupTests';

const setupArgoCd = (baseUrl: string | undefined) => {
    const argo: GetFrontendConfigResponse_ArgoCD | undefined = baseUrl
        ? {
              baseUrl: baseUrl,
          }
        : undefined;
    UpdateFrontendConfig.set({
        configs: {
            argoCd: argo,
            authConfig: undefined,
            kuberpultVersion: 'dontcare',
            manifestRepoUrl: 'dontcare',
            sourceRepoUrl: 'mysource',
            branch: 'dontcare',
        },
    });
};

describe('ArgoTeamLink', () => {
    const cases: {
        name: string;
        team: string | undefined;
        baseUrl: string | undefined;
    }[] = [
        {
            name: 'with team, without url',
            team: 'foo',
            baseUrl: undefined,
        },
        {
            name: 'with team, with url',
            team: 'foo',
            baseUrl: 'https://example.com/argo/',
        },
        {
            name: 'without team, with url',
            team: undefined,
            baseUrl: 'https://example.com/argo/',
        },
    ];
    describe.each(cases)('Renders properly', (testcase) => {
        const getNode = () => <ArgoTeamLink team={testcase.team} />;
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            //given
            setupArgoCd(testcase.baseUrl);
            getWrapper();
            // when
            // then
            expect(document.body).toMatchSnapshot();
        });
    });
});

describe('ArgoAppEnvLink', () => {
    const cases: {
        name: string;
        app: string;
        env: string;
        baseUrl: string | undefined;
    }[] = [
        {
            name: ' without url',
            app: 'foo',
            env: 'dev',
            baseUrl: undefined,
        },
        {
            name: ' with url',
            app: 'foo',
            env: 'dev',
            baseUrl: 'https://example.com/argo/',
        },
    ];
    describe.each(cases)('Renders properly', (testcase) => {
        const getNode = () => <ArgoAppEnvLink app={testcase.app} env={testcase.env} />;
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            //given
            setupArgoCd(testcase.baseUrl);
            getWrapper();
            // when
            // then
            expect(document.body).toMatchSnapshot();
        });
    });
});

describe('ArgoAppLink', () => {
    const cases: {
        name: string;
        app: string;
        baseUrl: string | undefined;
    }[] = [
        {
            name: 'without url',
            app: 'foo',
            baseUrl: undefined,
        },
        {
            name: 'with url',
            app: 'foo',
            baseUrl: 'https://example.com/argo/',
        },
    ];
    describe.each(cases)('Renders properly', (testcase) => {
        const getNode = () => <ArgoAppLink app={testcase.app} />;
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            //given
            setupArgoCd(testcase.baseUrl);
            getWrapper();
            // when
            // then
            expect(document.body).toMatchSnapshot();
        });
    });
});

const setupSourceRepo = (baseUrl: string) => {
    UpdateFrontendConfig.set({
        configs: {
            argoCd: undefined,
            authConfig: undefined,
            kuberpultVersion: 'kuberpult',
            manifestRepoUrl: 'mymanifest',
            sourceRepoUrl: baseUrl,
            branch: 'main',
        },
    });
};

const setupManifestRepo = (baseUrl: string) => {
    UpdateFrontendConfig.set({
        configs: {
            argoCd: undefined,
            authConfig: undefined,
            kuberpultVersion: 'kuberpult',
            manifestRepoUrl: baseUrl,
            sourceRepoUrl: 'mysource',
            branch: 'main',
        },
    });
};

describe('DisplayManifestLink', () => {
    const cases: {
        name: string;
        displayVersion: string;
        version: number;
        app: string;
        sourceRepo: string;
        expectedLink: string | undefined;
    }[] = [
        {
            name: 'Test with displayVersion',
            displayVersion: '1',
            version: 1,
            app: 'foo',
            sourceRepo: 'https://example.com/testing/{dir}/{branch}',
            expectedLink: 'https://example.com/testing/applications/foo/releases/1/main',
        },
        {
            name: 'Test without DisplayVersion',
            displayVersion: '',
            version: 1,
            app: 'foo',
            sourceRepo: 'https://example.com/testing/{branch}/{dir}',
            expectedLink: 'https://example.com/testing/main/applications/foo/releases/1',
        },
        {
            name: 'Test without repo link should render nothing',
            displayVersion: '1',
            version: 1,
            app: 'foo',
            sourceRepo: '',
            expectedLink: undefined,
        },
        {
            name: 'Test with undeployVersion should render nothing',
            displayVersion: '1',
            version: 0,
            app: 'foo',
            sourceRepo: 'https://example.com/testing',
            expectedLink: undefined,
        },
    ];

    describe.each(cases)('RendersProperly', (testcase) => {
        const getNode = () => (
            <DisplayManifestLink
                displayString={testcase.displayVersion}
                version={testcase.version}
                app={testcase.app}
            />
        );
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            setupManifestRepo(testcase.sourceRepo);
            const { container } = getWrapper();

            if (testcase.expectedLink) {
                // Either render the link:
                const aElem = elementQuerySelectorSafe(container, 'a');
                expect(aElem.attributes.getNamedItem('href')?.value).toBe(testcase.expectedLink);
            } else {
                // or render nothing:
                expect(document.body.textContent).toBe('');
            }
        });
    });
});

describe('DisplaySourceLink', () => {
    const cases: {
        name: string;
        displayVersion: string;
        commitId: string;
        sourceRepo: string;
        expectedLink: string | undefined;
    }[] = [
        {
            name: 'Test with displayVersion',
            displayVersion: '1',
            commitId: '123',
            sourceRepo: 'https://example.com/testing/{commit}/{branch}',
            expectedLink: 'https://example.com/testing/123/main',
        },
        {
            name: 'Test without DisplayVersion',
            displayVersion: '',
            commitId: '123',
            sourceRepo: 'https://example.com/testing/{branch}/{commit}',
            expectedLink: 'https://example.com/testing/main/123',
        },
        {
            name: 'Test without repo link should render nothing',
            displayVersion: '1',
            commitId: '123',
            sourceRepo: '',
            expectedLink: undefined,
        },
    ];

    describe.each(cases)('RendersProperly', (testcase) => {
        const getNode = () => (
            <DisplaySourceLink displayString={testcase.displayVersion} commitId={testcase.commitId} />
        );
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            setupSourceRepo(testcase.sourceRepo);
            const { container } = getWrapper();

            if (testcase.expectedLink) {
                // Either render the link:
                const aElem = elementQuerySelectorSafe(container, 'a');
                expect(aElem.attributes.getNamedItem('href')?.value).toBe(testcase.expectedLink);
            } else {
                // or render nothing:
                expect(document.body.textContent).toBe('');
            }
        });
    });
});

describe('DisplayCommitHistoryLink', () => {
    const cases: {
        name: string;
        commitId: string;
        expectedLink: string | undefined;
    }[] = [
        {
            name: 'Test with displayString',
            commitId: '123',
            expectedLink: '/ui/commits/123',
        },
        {
            name: 'Test Without commit should render nothing',
            commitId: '',
            expectedLink: undefined,
        },
    ];

    describe.each(cases)('RendersProperly', (testcase) => {
        const getNode = () => <DisplayCommitHistoryLink displayString={''} commitId={testcase.commitId} />;
        const getWrapper = () => render(getNode());
        it(testcase.name, () => {
            const { container } = getWrapper();

            if (testcase.expectedLink) {
                // Either render the link:
                const aElem = elementQuerySelectorSafe(container, 'a');
                expect(aElem.attributes.getNamedItem('href')?.value).toBe(testcase.expectedLink);
            } else {
                // or render nothing:
                expect(document.body.textContent).toBe('');
            }
        });
    });
});

describe('KuberpultGitHubLink', () => {
    const cases: {
        version: string;
        expectedLink: string;
    }[] = [
        {
            version: '2.6.0',
            expectedLink: 'https://github.com/freiheit-com/kuberpult/blob/v2.6.0/README.md',
        },
        {
            version: '6.6.6',
            expectedLink: 'https://github.com/freiheit-com/kuberpult/blob/v6.6.6/README.md',
        },
    ];
    describe.each(cases)('Renders properly', (testcase) => {
        const getNode = () => <KuberpultGitHubLink version={testcase.version} />;
        const getWrapper = () => render(getNode());
        it(testcase.version, () => {
            //given
            const { container } = getWrapper();
            // when
            const aElem = elementQuerySelectorSafe(container, 'a');
            // then
            expect(aElem.attributes.getNamedItem('href')?.value).toBe(testcase.expectedLink);
        });
    });
});
