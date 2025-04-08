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
import * as api from '../../api/api';

export interface Api {
    // overview service is used to get data about apps, envs and releases
    overviewService(): api.OverviewService;
    // batch service can make changes, e.g. adding/removing locks, deploying..
    batchService(): api.BatchService;
    // config service returns basics configuration like for azure auth
    configService(): api.FrontendConfigService;
    // rollout service
    rolloutService(): api.RolloutService;
    // tags service
    gitService(): api.GitService;
    // environment service
    environmentService(): api.EnvironmentService;
    // version service
    versionService(): api.VersionService;
}

class GrpcApi implements Api {
    _overviewService: api.OverviewService;
    _batchService: api.BatchService;
    _configService: api.FrontendConfigService;
    _rolloutService: api.RolloutService;
    _gitService: api.GitService;
    _environmentService: api.EnvironmentService;
    _releaseTrainPrognosisService: api.ReleaseTrainPrognosisService;
    _eslService: api.EslService;
    _versionService: api.VersionService;
    constructor() {
        // eslint-disable-next-line no-restricted-globals
        const gcli = new api.GrpcWebImpl(location.protocol + '//' + location.host, {});
        this._overviewService = new api.OverviewServiceClientImpl(gcli);
        this._batchService = new api.BatchServiceClientImpl(gcli);
        this._configService = new api.FrontendConfigServiceClientImpl(gcli);
        this._rolloutService = new api.RolloutServiceClientImpl(gcli);
        this._gitService = new api.GitServiceClientImpl(gcli);
        this._environmentService = new api.EnvironmentServiceClientImpl(gcli);
        this._releaseTrainPrognosisService = new api.ReleaseTrainPrognosisServiceClientImpl(gcli);
        this._eslService = new api.EslServiceClientImpl(gcli);
        this._versionService = new api.VersionServiceClientImpl(gcli);
    }
    overviewService(): api.OverviewService {
        return this._overviewService;
    }
    batchService(): api.BatchService {
        return this._batchService;
    }
    configService(): api.FrontendConfigService {
        return this._configService;
    }
    rolloutService(): api.RolloutService {
        return this._rolloutService;
    }
    gitService(): api.GitService {
        return this._gitService;
    }
    environmentService(): api.EnvironmentService {
        return this._environmentService;
    }
    releaseTrainPrognosisService(): api.ReleaseTrainPrognosisService {
        return this._releaseTrainPrognosisService;
    }
    eslService(): api.EslService {
        return this._eslService;
    }
    versionService(): api.VersionService {
        return this._versionService;
    }
}

export const useApi = new GrpcApi();
