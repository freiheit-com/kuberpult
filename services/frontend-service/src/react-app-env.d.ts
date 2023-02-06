/// <reference types="react-scripts" />
declare module NodeJS {
    interface Global {
        nextTick: () => Promise<void>;
    }
}
