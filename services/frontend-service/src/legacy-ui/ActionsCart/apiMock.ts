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
import { Environment_Application_ArgoCD, GetOverviewResponse } from '../../api/api';
import { CartAction } from '../ActionDetails';

export const mockGetOverviewResponseForActions = (
    actions: CartAction[],
    argoCD?: Environment_Application_ArgoCD
): GetOverviewResponse =>
    actions.reduce((response, action) => {
        if ('deploy' in action) {
            const environmentName = action.deploy.environment;
            const applicationName = action.deploy.application;
            return {
                ...response,
                environments: {
                    ...response.environments,
                    [environmentName]: {
                        ...(response.environments && response.environments[environmentName]),
                        name: environmentName,
                        applications: {
                            ...(response.environments && response.environments[environmentName]?.applications),
                            [applicationName]: {
                                name: applicationName,
                                version: 0,
                                locks: {},
                                queuedVersion: 0,
                                undeployVersion: false,
                                argoCD,
                            },
                        },
                    },
                },
            };
        } else {
            return response;
        }
    }, {} as GetOverviewResponse);
