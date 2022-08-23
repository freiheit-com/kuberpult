import React from 'react';
import { render } from '@testing-library/react';
import { NavbarIndicator } from './navListItem';

describe('Display sidebar indicator', () => {
    interface dataT {
        name: string;
        pathname: string;
        to: string;
        expect: (container: HTMLElement, url?: string) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'Indicator is not displayed',
            pathname: '/v2/test/',
            to: 'anotherTest',
            expect: (container) =>
                expect(container.querySelector(`.mdc-list-item-indicator--activated`)).not.toBeTruthy(),
        },
        {
            name: 'Indicator is displayed',
            pathname: '/v2/test/',
            to: 'test',
            expect: (container) => expect(container.querySelector(`.mdc-list-item-indicator--activated`)).toBeTruthy(),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <NavbarIndicator {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { pathname: string; to: string }) => render(getNode(overrides));

    describe.each(data)(`Sidebar Indicator Cases`, (testcase) => {
        it(testcase.name, () => {
            const { pathname, to } = testcase;
            // when
            const { container } = getWrapper({ pathname: pathname, to: to });
            // then
            testcase.expect(container);
        });
    });
});
