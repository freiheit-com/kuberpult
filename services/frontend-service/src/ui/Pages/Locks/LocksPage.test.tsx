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
import { render } from '@testing-library/react';
import { LocksPage } from './LocksPage';
import { DisplayLock } from '../../../api/api';
import React from 'react';
import { LocksTable } from '../../components/LocksTable/LocksTable';

const filterLocks = (locks: DisplayLock[], queryContent: string | null) =>
    locks.filter((val) => {
        if (queryContent) {
            if (val.application?.includes(queryContent)) {
                return val;
            }
            return null;
        } else {
            return val;
        }
    });

describe('LocksPage', () => {
    const getNode = (): JSX.Element | any => <LocksPage />;
    const getWrapper = () => render(getNode());

    it('Renders full app', () => {
        const { container } = getWrapper();
        expect(container.getElementsByClassName('mdc-data-table')[0]).toHaveTextContent('Environment Locks');
        expect(container.getElementsByClassName('mdc-data-table')[1]).toHaveTextContent('Application Locks');
    });
});

describe('Test Filter for Locks Table', () => {
    interface dataT {
        name: string;
        locks: DisplayLock[];
        query: string;
        headerTitle: string;
        columnHeaders: string[];
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <LocksTable {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides: { locks: DisplayLock[]; columnHeaders: string[]; headerTitle: string }) =>
        render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'two application locks pass the filter',
            locks: [
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app',
                    lockId: 'test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app',
                    lockId: 'another test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
            ] as DisplayLock[],
            query: '',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(2),
        },
        {
            name: 'only one application locks passes the filter',
            locks: [
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app',
                    lockId: 'test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app-v2',
                    lockId: 'another test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
            ] as DisplayLock[],
            query: 'v2',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(1),
        },
        {
            name: 'only one application locks passes the filter',
            locks: [
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app',
                    lockId: 'test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
                {
                    date: new Date(),
                    environment: 'test-env',
                    application: 'test-app-v2',
                    lockId: 'another test-id',
                    message: 'test-message',
                    authorName: 'defaultUser',
                    authorEmail: 'testEmail.com',
                },
            ] as DisplayLock[],
            query: 'v1',
            headerTitle: 'test-title',
            columnHeaders: ['Date', 'Environment', 'Lock Id', 'Message', 'Author Name', 'Author Email', ''],
            expect: (container) => expect(container.getElementsByClassName('lock-display')).toHaveLength(0),
        },
    ];

    describe.each(sampleApps)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const filteredLocks = filterLocks(testcase.locks, testcase.query);
            const { container } = getWrapper({
                locks: filteredLocks,
                columnHeaders: testcase.columnHeaders,
                headerTitle: testcase.headerTitle,
            });
            testcase.expect(container);
        });
    });
});
