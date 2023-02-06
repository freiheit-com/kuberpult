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
