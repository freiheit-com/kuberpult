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
/**
 * The RefreshStore is solution to refresh the state (GetOverview).
 * This is currently needed as an addition to polling.
 * This will be re-worked in v2.
 * See story SRX-0ELJF9.
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
