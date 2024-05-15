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
import { render } from '@testing-library/react';
import React from 'react';
import { LockDisplay } from '../LockDisplay/LockDisplay';
import { DisplayLock } from '../../utils/store';

describe('Run Locks Table', () => {
    interface dataT {
        name: string;
        lock: DisplayLock;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <LockDisplay {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides: { lock: DisplayLock }) => render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'one normal application lock',
            lock: {
                date: new Date(),
                environment: 'test-env',
                application: 'test-app',
                lockId: 'test-id',
                message: 'test-message',
                authorName: 'defaultUser',
                authorEmail: 'testEmail.com',
            },
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one normal environment lock',
            lock: {
                date: new Date(),
                environment: 'test-env',
                lockId: 'test-id',
                message: 'test-message',
                authorName: 'defaultUser',
                authorEmail: 'testEmail.com',
            },
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one outadeted application lock',
            lock: {
                date: new Date('1995-12-17T03:24:00'),
                environment: 'test-env',
                application: 'test-app',
                lockId: 'test-id',
                message: 'test-message',
                authorName: 'defaultUser',
                authorEmail: 'testEmail.com',
            },
            expect: (container) => expect(container.getElementsByClassName('date-display--outdated')).toHaveLength(1),
        },
        {
            name: 'one outdated existing lock',
            lock: {
                date: new Date('1995-12-17T03:24:00'),
                environment: 'test-env',
                application: 'test-app',
                lockId: 'test-id',
                message: 'test-message',
                authorName: 'defaultUser',
                authorEmail: 'testEmail.com',
            },
            expect: (container) => expect(container.getElementsByClassName('date-display--outdated')).toHaveLength(1),
        },
    ];

    describe.each(sampleApps)(`Renders an Application Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const { container } = getWrapper({ lock: testcase.lock });
            testcase.expect(container);
        });
    });
});
