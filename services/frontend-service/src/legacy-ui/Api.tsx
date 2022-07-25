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
import * as React from 'react';
import { Observable } from 'rxjs';

import * as api from '../api/api';

export interface Api {
    overviewService(): api.OverviewService;
    deployService(): api.DeployService;
    lockService(): api.LockService;
    batchService(): api.BatchService;
    configService(): api.FrontendConfigService;
}

const DummyApi: Api = {
    overviewService(): api.OverviewService {
        throw new Error('overviewService is unimplemented');
    },
    deployService(): api.DeployService {
        throw new Error('deployService is unimplemented');
    },
    lockService(): api.LockService {
        throw new Error('lockService is unimplemented');
    },
    batchService(): api.BatchService {
        throw new Error('batchService is unimplemented');
    },
    configService(): api.FrontendConfigService {
        throw new Error('configService is unimplemented');
    },
};

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

export const Context = React.createContext<Api>(DummyApi);
export const Provider = Context.Provider;
export const GrpcProvider = (props: { children: React.ReactNode }) => {
    const [api] = React.useState<Api>(new GrpcApi());
    return <Provider value={api}>{props.children}</Provider>;
};

/*
pending = not yet resolved
resolved = resolved succesfully
rejected = resolved with an error
*/
export type UnaryState<T> = { result: T; state: 'resolved' } | { error: any; state: 'rejected' } | { state: 'pending' };

export function useUnary<T>(callback: (api: Api) => Promise<T>): UnaryState<T> {
    const api = React.useContext(Context);
    const [state, setState] = React.useState<UnaryState<T>>({ state: 'pending' });
    React.useEffect(() => {
        callback(api).then(
            (result) => setState({ result, state: 'resolved' }),
            (error) => setState({ error, state: 'rejected' })
        );
    }, [api, callback]);
    return state;
}

/*
contains all states from UnaryState and one additional state that is used when the promise was not yet created
*/
export type UnaryCallbackState<T> = UnaryState<T> | { state: 'waiting' };

export function useUnaryCallback<T>(callback: (api: Api) => Promise<T>): [() => void, UnaryCallbackState<T>] {
    const api = React.useContext(Context);
    const [state, setState] = React.useState<UnaryCallbackState<T>>({ state: 'waiting' });
    const cb = React.useCallback(() => {
        setState({ state: 'pending' });
        callback(api).then(
            (result) => setState({ result, state: 'resolved' }),
            (error) => setState({ error, state: 'rejected' })
        );
    }, [api, callback]);
    return [cb, state];
}

export function useObservable<T>(callback: (api: Api) => Observable<T>): UnaryState<T> {
    const api = React.useContext(Context);
    const [state, setState] = React.useState<UnaryState<T>>({ state: 'pending' });
    React.useEffect(() => {
        const subscription = callback(api).subscribe(
            (result) => setState({ result, state: 'resolved' }),
            (error) => setState({ error, state: 'rejected' })
        );
        return () => subscription.unsubscribe();
    }, [api, callback]);
    return state;
}
