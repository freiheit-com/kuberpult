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
import { ArgoAppEnvLink, ArgoAppLink, ArgoTeamLink } from './Links';
import { GetFrontendConfigResponse_ArgoCD } from '../../api/api';
import { UpdateFrontendConfig } from './store';

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
            sourceRepoUrl: 'dontcare',
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
