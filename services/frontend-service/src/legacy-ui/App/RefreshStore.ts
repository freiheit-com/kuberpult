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
