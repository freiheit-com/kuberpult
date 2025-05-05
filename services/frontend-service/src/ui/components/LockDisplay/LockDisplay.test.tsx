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

Copyright freiheit.com*/
import { act, render } from '@testing-library/react';
import { Spy } from 'spy4js';
import { DisplayLock, displayLockUniqueId } from '../../utils/store';
import { isOutdated, LockDisplay } from './LockDisplay';
import { documentQuerySelectorSafe } from '../../../setupTests';
import { BrowserRouter } from 'react-router-dom';
const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

describe('Test Auxiliary Functions for Lock Display', () => {
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
                jest.useFakeTimers();
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
        ciLink: '',
        suggestedLifetime: '',
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
        {
            name: 'Test team lock delete button',
            date: new Date(2022, 0, 2),
            lock: { ...lock, team: 'test-team' },
        },
    ];

    describe.each(data)('lock type', (testcase) => {
        it(testcase.name, () => {
            render(<LockDisplay lock={testcase.lock} />);
            const result = documentQuerySelectorSafe('.service-action--delete');
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
            } else if (testcase.lock.team) {
                expect(JSON.stringify(mock_addAction.addAction.getAllCallArguments()[0][0])).toContain(
                    'deleteEnvironmentTeamLock'
                );
            } else {
                expect(JSON.stringify(mock_addAction.addAction.getAllCallArguments()[0][0])).toContain(
                    'deleteEnvironmentLock'
                );
            }
        });
    });
});

describe('Test link to ciLink', () => {
    interface dataT {
        name: string;
        lock: DisplayLock;
        date: Date;
    }
    const lockWithLink = {
        environment: 'test-env',
        lockId: 'test-lock-id-2',
        message: 'test-lock-123',
        ciLink: 'somelink',
        suggestedLifetime: '',
    };
    const lockWithoutLink = {
        environment: 'test-env',
        lockId: 'test-lock-id',
        message: 'test-lock-123',
        ciLink: '',
        suggestedLifetime: '',
    };
    const data: dataT[] = [
        {
            name: 'Environment Lock with Link',
            date: new Date(2022, 0, 2),
            lock: lockWithLink,
        },
        {
            name: 'App Lock with Link',
            date: new Date(2022, 0, 2),
            lock: { ...lockWithLink, application: 'test-app' },
        },
        {
            name: 'Team Lock with Link',
            date: new Date(2022, 0, 2),
            lock: { ...lockWithLink, team: 'test-team' },
        },
        {
            name: 'Environment Lock without Link',
            date: new Date(2022, 0, 2),
            lock: lockWithoutLink,
        },
    ];

    describe.each(data)('ci links on locks', (testcase) => {
        it(testcase.name, () => {
            render(
                <BrowserRouter>
                    <LockDisplay key={displayLockUniqueId(testcase.lock)} lock={testcase.lock} />{' '}
                </BrowserRouter>
            );
            expect(document.getElementsByClassName('lock-ci-link')).toHaveLength(testcase.lock.ciLink !== '' ? 1 : 0);
        });
    });
});

describe('Test suggested lifetime', () => {
    interface dataT {
        name: string;
        lock: DisplayLock;
    }
    const lockWithSuggestedLifetime = {
        environment: 'test-env',
        lockId: 'test-lock-id-2',
        message: 'test-lock-123',
        ciLink: '',
        suggestedLifetime: '1d',
        date: new Date(),
    };
    const lockWithoutSL = {
        environment: 'test-env',
        lockId: 'test-lock-id',
        message: 'test-lock-123',
        ciLink: '',
        suggestedLifetime: '',
    };
    const data: dataT[] = [
        {
            name: 'Environment Lock with lifetime',
            lock: lockWithSuggestedLifetime,
        },
        {
            name: 'App Lock with lifetime',
            lock: { ...lockWithSuggestedLifetime, application: 'test-app' },
        },
        {
            name: 'Team Lock with lifetime',
            lock: { ...lockWithSuggestedLifetime, team: 'test-team' },
        },
        {
            name: 'Environment Lock without lifetime',
            lock: lockWithoutSL,
        },
    ];

    describe.each(data)('suggested lifetime on locks', (testcase) => {
        it(testcase.name, () => {
            const { container } = render(
                <BrowserRouter>
                    <LockDisplay key={displayLockUniqueId(testcase.lock)} lock={testcase.lock} />{' '}
                </BrowserRouter>
            );
            const result = document.getElementsByClassName('lifetime-date');
            expect(result[0]).toHaveTextContent(testcase.lock.suggestedLifetime ? 'in 24 hours' : '-');
        });
    });
});
