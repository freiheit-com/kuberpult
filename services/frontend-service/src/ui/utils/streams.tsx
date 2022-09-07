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

import * as api from '../../api/api';
import * as React from 'react';
import { panicOverview, UpdateOverview } from './store';
import { Observable } from 'rxjs';
import { UnaryState } from '../../legacy-ui/Api';

const GetOverview = () => {
    const getOverview = React.useCallback((api) => api.overviewService().StreamOverview({}), []);
    const overview = useObservable<api.GetOverviewResponse>(getOverview);
    switch (overview.state) {
        case 'resolved':
            UpdateOverview.set(overview.result);
            return;
        case 'rejected':
            panicOverview.set(overview.error);
            return;
        case 'pending':
            return;
        default:
            return;
    }
};

export function useObservable<T>(callback: () => Observable<T>): UnaryState<T> {
    const [state, setState] = React.useState<UnaryState<T>>({ state: 'pending' });
    React.useEffect(() => {
        const subscription = callback().subscribe(
            (result) => setState({ result, state: 'resolved' }),
            (error) => setState({ error, state: 'rejected' })
        );
        return () => subscription.unsubscribe();
    }, [api, callback]);
    return state;
}
