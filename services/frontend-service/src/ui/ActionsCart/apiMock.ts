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
