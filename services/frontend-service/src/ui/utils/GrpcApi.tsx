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
import * as api from '../../api/api';

export interface Api {
    overviewService(): api.OverviewService;
    deployService(): api.DeployService;
    lockService(): api.LockService;
    batchService(): api.BatchService;
    configService(): api.FrontendConfigService;
}

class GrpcApi implements Api {
    _overviewService: api.OverviewService;
    _deployService: api.DeployService;
    _lockService: api.LockService;
    _batchService: api.BatchService;
    _configService: api.FrontendConfigService;
    constructor() {
        // eslint-disable-next-line no-restricted-globals
        const gcli = new api.GrpcWebImpl(location.protocol + '//' + location.host, {});
        this._overviewService = new api.OverviewServiceClientImpl(gcli);
        this._deployService = new api.DeployServiceClientImpl(gcli);
        this._lockService = new api.LockServiceClientImpl(gcli);
        this._batchService = new api.BatchServiceClientImpl(gcli);
        this._configService = new api.FrontendConfigServiceClientImpl(gcli);
    }
    overviewService(): api.OverviewService {
        return this._overviewService;
    }
    deployService(): api.DeployService {
        return this._deployService;
    }
    lockService(): api.LockService {
        return this._lockService;
    }
    batchService(): api.BatchService {
        return this._batchService;
    }
    configService(): api.FrontendConfigService {
        return this._configService;
    }
}

export const useApi = new GrpcApi();
