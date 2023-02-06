import { calcLockAge, daysToString, isOutdated } from '../LockDisplay/LockDisplay';

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
                date: new Date('2/1/22'),
                expected: 0,
            },
            {
                name: 'working for different dates',
                date: new Date('1/15/22'),
                expected: 17,
            },
        ];

        describe.each(cases)(`Tests each calcLocksAge`, (testcase) => {
            beforeAll(() => {
                jest.useFakeTimers('modern');
                jest.setSystemTime(new Date('2/1/22'));
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
                date: new Date('2/1/22'),
                expected: false,
            },
            {
                name: 'working for outdated lock',
                date: new Date('1/15/22'),
                expected: true,
            },
        ];

        describe.each(cases)(`Tests each isOutdated`, (testcase) => {
            beforeAll(() => {
                jest.useFakeTimers('modern');
                jest.setSystemTime(new Date('2/1/22'));
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
