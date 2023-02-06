import '@testing-library/jest-dom/extend-expect';
import 'react-use-sub/test-util';

// test utility to await all running promises
global.nextTick = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 0));
