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
import {
    BatchAction,
    BatchService,
    DeepPartial,
    DeployService,
    Environment_Application_ArgoCD,
    GetOverviewRequest,
    GetOverviewResponse,
    LockService,
    OverviewService,
} from '../../api/api';
import { Observable } from 'rxjs';
import { StateOfUnaryState } from '../Api';

const mockGetOverviewResponseForActions = (
    actions: BatchAction[],
    argoCD?: Environment_Application_ArgoCD
): GetOverviewResponse =>
    actions.reduce((response, action) => {
        switch (action.action?.$case) {
            case 'deploy':
                const environmentName = action.action.deploy.environment;
                const applicationName = action.action.deploy.application;
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
            default:
                return response;
        }
    }, {} as GetOverviewResponse);
export const makeApiMock = (
    actions: BatchAction[],
    getOverviewState: StateOfUnaryState = 'pending',
    argoCD?: Environment_Application_ArgoCD
) => {
    const getOverviewResponse = mockGetOverviewResponseForActions(actions, argoCD);
    return {
        overviewService(): OverviewService {
            return {
                GetOverview: (_: DeepPartial<GetOverviewRequest>) =>
                    new Promise((resolve, reject) => {
                        switch (getOverviewState) {
                            case 'resolved':
                                return resolve(getOverviewResponse);
                            case 'rejected':
                                return reject();
                        }
                    }),
                StreamOverview: (_: DeepPartial<GetOverviewRequest>) => new Observable<GetOverviewResponse>(),
            };
        },
        deployService(): DeployService {
            throw new Error('deployService is unimplemented');
        },
        lockService(): LockService {
            throw new Error('lockService is unimplemented');
        },
        batchService(): BatchService {
            throw new Error('batchService is unimplemented');
        },
    };
};
