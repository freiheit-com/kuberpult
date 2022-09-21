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
import React from 'react';
import { ApplicationLockDisplay } from '../ApplicationLockDisplay/ApplicationLockDisplay';

describe('Application Card', () => {
    interface dataT {
        name: string;
        lock: (Date | undefined | string)[];
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <ApplicationLockDisplay {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides: { lock: (Date | undefined | string)[] }) => render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'no existing locks',
            lock: [],
            expect: (container) =>
                expect(
                    container.getElementsByClassName('app-lock-display-info date-display--normal')[0]
                ).toBeEmptyDOMElement(),
        },
        {
            name: 'one existing lock',
            lock: ['asd', 'asda', 'asdas'],
            expect: (container) => expect(container.getElementsByClassName('app-lock-display')).toHaveLength(1),
        },
        {
            name: 'one existing lock with normal date',
            lock: [new Date(), 'asda', 'asdas'],
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one outdated existing lock',
            lock: [new Date('1995-12-17T03:24:00'), 'asda', 'asdas'],
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
