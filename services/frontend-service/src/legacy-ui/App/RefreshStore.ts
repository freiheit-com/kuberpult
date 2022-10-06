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
