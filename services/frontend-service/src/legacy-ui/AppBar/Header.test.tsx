import React from 'react';
import { getByTestId, render } from '@testing-library/react';
import { HeaderTitle } from './Header';

describe('Show Kuberpult Version', () => {
    interface dataT {
        name: string;
        tooltipText: string;
        expect: (container: HTMLElement) => HTMLElement | void;
    }

    const data: dataT[] = [
        {
            name: 'renders the Tooltip component without version',
            tooltipText: '',
            expect: (container) =>
                expect(getByTestId(container, 'kuberpult-version')).toHaveAttribute('aria-label', 'Kuberpult '),
        },
        {
            name: 'renders the Tooltip component with version',
            tooltipText: '1.0.0',
            expect: (container) =>
                expect(getByTestId(container, 'kuberpult-version')).toHaveAttribute('aria-label', 'Kuberpult 1.0.0'),
        },
    ];

    const getNode = (overrides?: {}): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
        };
        return <HeaderTitle {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { kuberpultVersion: string }) => render(getNode(overrides));

    describe.each(data)(`Kuberpult Version UI`, (testcase) => {
        it(testcase.name, () => {
            const { tooltipText } = testcase;
            // when
            const { container } = getWrapper({ kuberpultVersion: tooltipText });
            // then
            testcase.expect(container);
        });
    });
});
