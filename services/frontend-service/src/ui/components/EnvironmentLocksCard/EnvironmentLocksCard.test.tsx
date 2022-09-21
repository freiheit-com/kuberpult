import { render } from '@testing-library/react';
import React from 'react';
import { EnvironmentLockDisplay } from '../EnvironmentLockDisplay/EnvironmentLockDisplay';

describe('Environment Card', () => {
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
        return <EnvironmentLockDisplay {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides: { lock: (Date | undefined | string)[] }) => render(getNode(overrides));

    const sampleApps: dataT[] = [
        {
            name: 'no existing locks',
            lock: [],
            expect: (container) =>
                expect(
                    container.getElementsByClassName('env-lock-display-info date-display--normal')[0]
                ).toBeEmptyDOMElement(),
        },
        {
            name: 'one existing lock',
            lock: ['asd', 'asda', 'asdas'],
            expect: (container) => expect(container.getElementsByClassName('env-lock-display')).toHaveLength(1),
        },
        {
            name: 'one existing lock',
            lock: [new Date(), 'asda', 'asdas'],
            expect: (container) => expect(container.getElementsByClassName('date-display--normal')).toHaveLength(1),
        },
        {
            name: 'one outdated existing lock',
            lock: [new Date('1995-12-17T03:24:00'), 'asda', 'asdas'],
            expect: (container) => expect(container.getElementsByClassName('date-display--outdated')).toHaveLength(1),
        },
    ];

    describe.each(sampleApps)(`Renders an Environment Card`, (testcase) => {
        it(testcase.name, () => {
            // when
            const { container } = getWrapper({ lock: testcase.lock });
            testcase.expect(container);
        });
    });
});
