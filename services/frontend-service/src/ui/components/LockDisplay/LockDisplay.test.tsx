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
import { act, render } from '@testing-library/react';
import { Spy } from 'spy4js';
import { DisplayLock } from '../../utils/store';
import { calcLockAge, daysToString, isOutdated, LockDisplay } from './LockDisplay';
const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

describe('Test Auxiliary Functions for Lock Display', () => {
    describe('Test daysToString', () => {
        interface dataT {
            name: string;
            daysString: string;
            expected: string;
        }
        const cases: dataT[] = [
            {
                name: 'valid Date, less than one day',
                daysString: daysToString(0),
                expected: '< 1 day ago',
            },
            {
                name: 'valid Date, one day',
                daysString: daysToString(1),
                expected: '1 day ago',
            },
            {
                name: 'valid Date, more than one day',
                daysString: daysToString(2),
                expected: '2 days ago',
            },
        ];

        describe.each(cases)(`Test each daysToString`, (testcase) => {
            it(testcase.name, () => {
                expect(testcase.expected).toBe(testcase.daysString);
            });
        });
    });

    describe('Test calcLockAge', () => {
        interface dataT {
            name: string;
            date: Date;
            expected: number;
        }
        const cases: dataT[] = [
            {
                name: 'working for same date',
                date: new Date(2022, 1, 1),
                expected: 0,
            },
            {
                name: 'working for different dates',
                date: new Date(2022, 0, 15),
                expected: 17,
            },
        ];

        describe.each(cases)(`Tests each calcLocksAge`, (testcase) => {
            beforeAll(() => {
                jest.useFakeTimers('modern');
                jest.setSystemTime(new Date(2022, 1, 1));
            });

            afterAll(() => {
                jest.useRealTimers();
            });

            it(testcase.name, () => {
                expect(calcLockAge(testcase.date)).toBe(testcase.expected);
            });
        });
    });

    describe('Test isOutdated', () => {
        interface dataT {
            name: string;
            date: Date;
            expected: boolean;
        }
        const cases: dataT[] = [
            {
                name: 'working for normal lock',
                date: new Date(2022, 1, 1),
                expected: false,
            },
            {
                name: 'working for outdated lock',
                date: new Date(2022, 0, 15),
                expected: true,
            },
        ];

        describe.each(cases)(`Tests each isOutdated`, (testcase) => {
            beforeAll(() => {
                jest.useFakeTimers('modern');
                jest.setSystemTime(new Date(2022, 1, 1));
            });

            afterAll(() => {
                jest.useRealTimers();
            });

            it(testcase.name, () => {
                expect(isOutdated(testcase.date)).toBe(testcase.expected);
            });
        });
    });
});

const querySelectorSafe = (selectors: string): HTMLElement => {
    const result = document.querySelector(selectors);
    if (!result) {
        throw new Error('did not find in selector in document ' + selectors);
    }
    if (!(result instanceof HTMLElement)) {
        throw new Error('did find element in selector but it is not an html element: ' + selectors);
    }
    return result;
};

describe('Test delete lock button', () => {
    interface dataT {
        name: string;
        lock: DisplayLock;
        date: Date;
    }
    const lock = {
        environment: 'test-env',
        lockId: 'test-lock-id',
        message: 'test-lock-123',
    };
    const data: dataT[] = [
        {
            name: 'Test environment lock delete button',
            date: new Date(2022, 0, 2),
            lock: lock,
        },
        {
            name: 'Test application lock delete button',
            date: new Date(2022, 0, 2),
            lock: { ...lock, application: 'test-app' },
        },
    ];

    describe.each(data)('lock type', (testcase) => {
        it(testcase.name, () => {
            render(<LockDisplay lock={testcase.lock} />);
            const result = querySelectorSafe('.service-action--delete');
            act(() => {
                result.click();
            });
            // then
            expect(JSON.stringify(mock_addAction.addAction.getAllCallArguments()[0][0])).toContain(
                testcase.lock.lockId
            );
            if (testcase.lock.application) {
                expect(JSON.stringify(mock_addAction.addAction.getAllCallArguments()[0][0])).toContain(
                    'deleteEnvironmentApplicationLock'
                );
            } else {
                expect(JSON.stringify(mock_addAction.addAction.getAllCallArguments()[0][0])).toContain(
                    'deleteEnvironmentLock'
                );
            }
        });
    });
});
