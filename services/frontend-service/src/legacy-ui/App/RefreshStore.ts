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
/**
 * The RefreshStore is solution to refresh the state (GetOverview).
 * This is currently needed as an addition to polling.
 * This will be re-worked in v2.
 */

class RefreshStore {
    doRefresh: boolean = false;

    setRefresh(newState: boolean) {
        this.doRefresh = newState;
    }

    shouldRefresh() {
        return this.doRefresh;
    }
}

const refreshStore = new RefreshStore();
export default refreshStore;
